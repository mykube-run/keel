package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/pb"
	"github.com/mykube-run/keel/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"net"
	"net/http"
	"time"
)

type Server struct {
	pb.UnimplementedScheduleServiceServer
	db     types.DB
	sched  *Scheduler
	config config.ServerConfig
	lg     types.Logger
	ls     types.Listener
}

func NewServer(db types.DB, sched *Scheduler, config config.ServerConfig, lg types.Logger, ls types.Listener) *Server {
	return &Server{db: db, sched: sched, config: config, lg: lg, ls: ls}
}

func (s *Server) NewTenant(ctx context.Context, req *pb.NewTenantRequest) (*pb.Response, error) {
	s.lg.Log(types.LevelInfo, "tenantId", req.Uid, "zone", req.Zone, "name", req.Name,
		"message", "creating new tenant")
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	now := time.Now()
	t := entity.Tenant{
		Uid:        req.Uid,
		Zone:       req.Zone,
		Priority:   req.Priority,
		Partition:  s.sched.SchedulerId(),
		Name:       req.Name,
		Status:     enum.TenantStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
		LastActive: now,
		ResourceQuota: entity.ResourceQuota{
			Concurrency: sql.NullInt64{
				Int64: req.Quota.Value,
				Valid: true,
			},
		},
	}
	if err := s.db.CreateTenant(ctx, t); err != nil {
		if errors.Is(err, enum.ErrTenantAlreadyExists) {
			err = status.Error(codes.AlreadyExists, enum.ErrTenantAlreadyExists.Error())
			s.lg.Log(types.LevelInfo, "tenantId", req.Uid, "message", "tenant already exists")
			return nil, err
		}
		s.lg.Log(types.LevelError, "error", err, "tenantId", req.Uid, "message", "failed to create tenant")
		return nil, err
	}
	s.lg.Log(types.LevelInfo, "tenantId", req.Uid, "message", "created new tenant")
	return resp, nil
}

func (s *Server) TenantTaskInfo(ctx context.Context, req *pb.TenantTaskRequest) (resp *pb.TenantTaskResponse, err error) {
	s.lg.Log(types.LevelDebug, "tenantId", req.TenantID, "message", "get tenant tasks")
	queue, ex := s.sched.cs[req.GetTenantID()]
	pendingCount, err := s.db.CountTenantPendingTasks(ctx, types.CountTenantPendingTasksOption{TenantId: req.TenantID})
	if err != nil {
		s.lg.Log(types.LevelError, "error", err, "tenantId", req.TenantID, "message", "failed to count tenant pending tasks")
		return nil, err
	}
	if ex {
		limit := queue.Tenant.ResourceQuota.Concurrency.Int64
		runningTasks := 0
		runningTasks, err = s.sched.em.CountRunningTasks(req.TenantID)
		if err != nil {
			resp = &pb.TenantTaskResponse{
				Code: pb.Code_TaskNotExist,
			}
			return
		}
		resp = &pb.TenantTaskResponse{
			Code: pb.Code_Ok, Running: int64(runningTasks), Limit: limit,
			Pending: pendingCount,
		}
		return
	} else {
		var tenant entity.Tenant
		tenant, err = s.db.GetTenant(ctx, types.GetTenantOption{TenantId: req.TenantID})
		if err != nil {
			s.lg.Log(types.LevelError, "error", err, "tenantId", req.TenantID, "message", "found tenant tasks, but tenant does not exist")
			resp = &pb.TenantTaskResponse{
				Code: pb.Code_TaskNotExist,
			}
			return
		}
		resp = &pb.TenantTaskResponse{
			Code: pb.Code_Ok, Running: 0, Pending: pendingCount,
			Limit: tenant.ResourceQuota.Concurrency.Int64,
		}
		return resp, nil
	}
}

func (s *Server) NewTask(ctx context.Context, req *pb.NewTaskRequest) (*pb.Response, error) {
	s.lg.Log(types.LevelInfo, "tenantId", req.TenantId, "taskId", req.Uid, "message", "creating new task")
	now := time.Now()
	t := entity.UserTask{
		TenantId:         req.TenantId,
		Uid:              req.Uid,
		Handler:          req.Handler,
		Config:           req.Config,
		ScheduleStrategy: req.ScheduleStrategy,
		Priority:         req.Priority,
		Progress:         0,
		CreatedAt:        now,
		UpdatedAt:        now,
		Status:           enum.TaskStatusPending,
	}
	if req.Options.GetCheckResourceQuota() {
		queue, ex := s.sched.cs[req.TenantId]
		if ex {
			limit := queue.Tenant.ResourceQuota.Concurrency.Int64
			runningTasks, err := s.sched.em.CountRunningTasks(req.TenantId)
			if err != nil {
				return nil, err
			}
			if int64(runningTasks) > limit {
				resp := &pb.Response{
					Code: pb.Code_TenantQuotaExceeded,
				}
				return resp, nil
			}
		}
	}
	if err := s.db.CreateTask(ctx, t); err != nil {
		s.lg.Log(types.LevelError, "error", err, "tenantId", req.TenantId, "taskId", req.Uid,
			"message", "failed to create new task")
		return nil, err
	}
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	s.lg.Log(types.LevelDebug, "tenantId", req.TenantId, "taskId", req.Uid,
		"message", "created new task")
	le := types.ListenerEvent{SchedulerId: s.sched.SchedulerId(), Task: types.NewTaskMetadataFromUserTaskEntity(&t)}
	s.ls.OnTaskCreated(le)
	return resp, nil
}

func (s *Server) PauseTask(ctx context.Context, req *pb.PauseTaskRequest) (*pb.Response, error) {
	taskStatus, err := s.db.GetTaskStatus(ctx, types.GetTaskStatusOption{Uid: req.Uid, TaskType: enum.TaskTypeUserTask})
	if taskStatus != enum.TaskStatusRunning {
		resp := &pb.Response{
			Code:    pb.Code_TaskIsNotRunning,
			Message: "The task is not running",
		}
		return resp, nil
	}
	err = s.db.UpdateTaskStatus(ctx, types.UpdateTaskStatusOption{
		Uids: []string{req.Uid}, TaskType: enum.TaskTypeUserTask,
		Status: enum.TaskStatusDispatched,
	})
	if err != nil {
		return nil, err
	}
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	return resp, nil
}

func (s *Server) RestartTask(ctx context.Context, req *pb.RestartTaskRequest) (*pb.Response, error) {
	s.lg.Log(types.LevelDebug, "taskId", req.Uid, "message", "restart task")
	panic("implement me")
}

func (s *Server) StopTask(ctx context.Context, req *pb.StopTaskRequest) (*pb.Response, error) {
	task, err := s.db.GetTask(ctx, types.GetTaskOption{TaskType: enum.TaskTypeUserTask, Uid: req.Uid})
	if err != nil {
		return nil, err
	}
	if len(task.UserTasks.TaskIds()) == 0 {
		resp := &pb.Response{
			Code:    pb.Code_TaskNotExist,
			Message: "task not found",
		}
		return resp, nil
	}
	findTask := task.UserTasks[0]
	if findTask.TenantId != req.TenantID {
		resp := &pb.Response{
			Code:    pb.Code_PermissionDenied,
			Message: "task not found in this tenant",
		}
		return resp, nil
	}
	if findTask.Status == enum.TaskStatusSuccess || findTask.Status == enum.TaskStatusFailed {
		resp := &pb.Response{
			Code:    pb.Code_TaskExecuted,
			Message: "The task has finished running",
		}
		return resp, nil
	}

	if err != nil {
		return nil, err
	}
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}

	return resp, nil
}

func (s *Server) QueryTaskStatus(ctx context.Context, req *pb.QueryTaskRequest) (*pb.QueryStatusResponse, error) {
	options := types.GetTaskStatusOption{Uid: req.Uid}
	taskStatus, err := s.db.GetTaskStatus(ctx, options)
	if err != nil {
		s.lg.Log(types.LevelError, "error", err, "taskId", req.Uid, "message", "query task error")
		return nil, err
	}
	resp := &pb.QueryStatusResponse{
		Code:   pb.Code_Ok,
		Status: string(taskStatus),
	}
	return resp, nil
}

func (s *Server) Start() {
	// Create a ls on TCP port
	lis, err := net.Listen("tcp", s.config.GrpcAddress)
	if err != nil {
		s.lg.Log(types.LevelFatal, "message", fmt.Sprintf("failed to listen: %v", err))
	}

	// Create a gRPC server object
	srv := grpc.NewServer()
	// Attach the Greeter service to the server
	pb.RegisterScheduleServiceServer(srv, s)
	// Serve gRPC server
	s.lg.Log(types.LevelInfo, "message", fmt.Sprintf("serving gRPC on %s", s.config.GrpcAddress))
	go func() {
		s.lg.Log(types.LevelFatal, "error", srv.Serve(lis))
	}()

	// Create a client connection to the gRPC server we just started
	// This is where the gRPC-Gateway proxies the requests
	conn, err := grpc.DialContext(
		context.Background(),
		s.config.GrpcAddress,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		s.lg.Log(types.LevelFatal, "message", fmt.Sprintf("failed to dial gRPC server: %v", err))
	}

	// Register gateway
	gwmux := runtime.NewServeMux()
	err = pb.RegisterScheduleServiceHandler(context.Background(), gwmux, conn)
	if err != nil {
		s.lg.Log(types.LevelFatal, "message", fmt.Sprintf("failed to register gateway: %v", err))
	}

	gwServer := &http.Server{
		Addr:    s.config.HttpAddress,
		Handler: gwmux,
	}
	s.lg.Log(types.LevelInfo, "message", fmt.Sprintf("serving gRPC-Gateway on http://%s", s.config.HttpAddress))
	s.lg.Log(types.LevelFatal, "error", gwServer.ListenAndServe())
}
