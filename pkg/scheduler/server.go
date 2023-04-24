package scheduler

import (
	"context"
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
	lg     types.Logger
	ls     types.Listener
	sched  *Scheduler
	config config.ServerConfig
}

func NewServer(db types.DB, sched *Scheduler, config config.ServerConfig, lg types.Logger, ls types.Listener) *Server {
	return &Server{db: db, sched: sched, config: config, lg: lg, ls: ls}
}

// Tenant API

func (s *Server) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.Response, error) {
	s.lg.Log(types.LevelInfo, "tenantId", req.GetUid(), "zone", req.GetZone(),
		"name", req.GetName(), "quota", req.GetQuota(), "message", "creating new tenant")

	now := time.Now()
	t := entity.Tenant{
		Uid:           req.GetUid(),
		Zone:          req.GetZone(),
		Priority:      req.GetPriority(),
		Partition:     s.sched.SchedulerId(),
		Name:          req.GetName(),
		Status:        enum.TenantStatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastActive:    now,
		ResourceQuota: newResourceQuota(req.GetUid(), req.GetQuota()),
	}
	err := s.db.CreateTenant(ctx, t)
	if err != nil {
		if errors.Is(err, enum.ErrTenantAlreadyExists) {
			err = status.Error(codes.AlreadyExists, enum.ErrTenantAlreadyExists.Error())
			s.lg.Log(types.LevelInfo, "tenantId", req.GetUid(), "message", "tenant already exists")
			return nil, err
		}
		s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", req.GetUid(), "message", "failed to create tenant")
		return nil, err
	}

	s.lg.Log(types.LevelInfo, "tenantId", req.GetUid(), "partition", t.Partition, "message", "created new tenant")
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	return resp, nil
}

func (s *Server) QueryTenantTaskConcurrency(ctx context.Context, req *pb.QueryTenantTaskConcurrencyRequest) (resp *pb.QueryTenantTaskConcurrencyResponse, err error) {
	var (
		running int
		limit   int64
	)
	s.lg.Log(types.LevelDebug, "tenantId", req.GetUid(), "message", "querying tenant task concurrency")

	// 1. Count pending tasks from database
	pending, err := s.db.CountTenantPendingTasks(ctx, types.CountTenantPendingTasksOption{TenantId: req.GetUid()})
	if err != nil {
		s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", req.GetUid(), "message", "failed to count tenant pending tasks")
		return nil, err
	}

	// 2. Get running and concurrency limit from queue or database
	queue, ok := s.sched.cs[req.GetUid()]
	if ok {
		limit = queue.Tenant.ResourceQuota.Concurrency
		running, err = s.sched.em.CountRunningTasks(req.GetUid())
		if err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", req.GetUid(), "message", "failed to count tenant running tasks")
			return nil, err
		}
	} else {
		var tenant *entity.Tenant
		tenant, err = s.db.GetTenant(ctx, types.GetTenantOption{TenantId: req.GetUid()})
		if err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", req.GetUid(), "message", "failed to get tenant")
			return nil, err
		}
		limit = tenant.ResourceQuota.Concurrency
		// TODO: running
	}

	resp = &pb.QueryTenantTaskConcurrencyResponse{
		Code:    pb.Code_Ok,
		Limit:   limit,
		Running: int64(running),
		Pending: pending,
	}
	return resp, nil
}

// Task API

func (s *Server) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Response, error) {
	s.lg.Log(types.LevelInfo, "tenantId", req.GetTenantId(), "taskId", req.GetUid(),
		"handler", req.GetHandler(), "message", "creating new task")

	now := time.Now()
	t := entity.Task{
		TenantId:         req.GetTenantId(),
		Uid:              req.GetUid(),
		Handler:          req.GetHandler(),
		Config:           req.GetConfig(),
		ScheduleStrategy: req.GetScheduleStrategy(),
		Priority:         req.GetPriority(),
		Progress:         0,
		CreatedAt:        now,
		UpdatedAt:        now,
		Status:           enum.TaskStatusPending,
	}
	if req.Options.GetCheckResourceQuota() {
		queue, ok := s.sched.cs[req.TenantId]
		if ok {
			limit := queue.Tenant.ResourceQuota.Concurrency
			running, err := s.sched.em.CountRunningTasks(req.TenantId)
			if err != nil {
				s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", req.GetTenantId(), "message", "failed to count running tasks while creating new task")
				return nil, err
			}
			if int64(running) > limit {
				resp := &pb.Response{
					Code: pb.Code_TenantQuotaExceeded,
				}
				return resp, nil
			}
		}
	}
	if err := s.db.CreateTask(ctx, t); err != nil {
		s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", req.GetTenantId(), "taskId", req.GetUid(),
			"message", "failed to create new task")
		return nil, err
	}
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	s.lg.Log(types.LevelDebug, "tenantId", req.GetTenantId(), "taskId", req.GetUid(),
		"handler", req.GetHandler(), "message", "created new task")
	le := types.ListenerEvent{SchedulerId: s.sched.SchedulerId(), Task: types.NewTaskMetadataFromTaskEntity(&t)}
	s.ls.OnTaskCreated(le)
	return resp, nil
}

func (s *Server) PauseTask(ctx context.Context, req *pb.PauseTaskRequest) (*pb.Response, error) {
	s.lg.Log(types.LevelInfo, "taskId", req.GetUid(), "taskType", req.GetType(), "message", "pausing task")

	opt := types.GetTaskStatusOption{
		TenantId: req.GetTenantId(),
		Uid:      req.GetUid(),
	}
	ts, err := s.db.GetTaskStatus(ctx, opt)
	if ts != enum.TaskStatusRunning {
		resp := &pb.Response{
			Code:    pb.Code_TaskNotRunning,
			Message: "task is not running",
		}
		return resp, nil
	}
	// TODO: pause running task
	err = s.db.UpdateTaskStatus(ctx, types.UpdateTaskStatusOption{
		TenantId: req.GetTenantId(),
		Uids:     []string{req.GetUid()},
		Status:   enum.TaskStatusCanceled,
	})
	if err != nil {
		s.lg.Log(types.LevelError, "error", err.Error(), "taskId", req.GetUid(), "message", "failed to update task status")
		return nil, err
	}
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	return resp, nil
}

func (s *Server) RestartTask(ctx context.Context, req *pb.RestartTaskRequest) (*pb.Response, error) {
	s.lg.Log(types.LevelInfo, "taskId", req.GetUid(), "taskType", req.GetType(), "message", "restarting task")

	err := s.db.UpdateTaskStatus(ctx, types.UpdateTaskStatusOption{
		TenantId: req.GetTenantId(),
		Uids:     []string{req.GetUid()},
		Status:   enum.TaskStatusPending,
	})
	if err != nil {
		s.lg.Log(types.LevelError, "error", err.Error(), "taskId", req.GetUid(), "message", "failed to update task status")
		return nil, err
	}
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	return resp, nil
}

func (s *Server) StopTask(ctx context.Context, req *pb.StopTaskRequest) (*pb.Response, error) {
	s.lg.Log(types.LevelInfo, "taskId", req.GetUid(), "taskType", req.GetType(), "message", "stopping task")

	opt := types.GetTaskStatusOption{
		TenantId: req.GetTenantId(),
		Uid:      req.GetUid(),
	}
	ts, err := s.db.GetTaskStatus(ctx, opt)
	if err != nil {
		s.lg.Log(types.LevelError, "taskId", req.GetUid(), "taskType", req.GetType(), "message", "failed to get task status while stopping task")
		return nil, err
	}
	if ts == enum.TaskStatusSuccess || ts == enum.TaskStatusFailed {
		resp := &pb.Response{
			Code:    pb.Code_TaskFinished,
			Message: fmt.Sprintf("task has been finished (status: %v)", ts),
		}
		return resp, nil
	}

	// TODO: stop task

	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	return resp, nil
}

func (s *Server) QueryTaskStatus(ctx context.Context, req *pb.QueryTaskStatusRequest) (*pb.QueryTaskStatusResponse, error) {
	opt := types.GetTaskStatusOption{
		TenantId: req.GetTenantId(),
		Uid:      req.GetUid(),
	}
	ts, err := s.db.GetTaskStatus(ctx, opt)
	if err != nil {
		s.lg.Log(types.LevelError, "error", err.Error(), "taskId", req.GetUid(), "message", "query task error")
		return nil, err
	}
	resp := &pb.QueryTaskStatusResponse{
		Code:   pb.Code_Ok,
		Status: string(ts),
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

func newResourceQuota(tid string, q *pb.ResourceQuota) entity.ResourceQuota {
	quota := entity.ResourceQuota{
		TenantId:    tid,
		Concurrency: q.GetConcurrency(),
		CPU:         q.GetCpu(),
		Custom:      q.GetCustom(),
		GPU:         q.GetGpu(),
		Memory:      q.GetMemory(),
		Storage:     q.GetStorage(),
		Peak:        q.GetPeak(),
	}
	return quota
}
