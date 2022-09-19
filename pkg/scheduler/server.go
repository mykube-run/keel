package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/pb"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"net"
	"net/http"
	"time"
)

type server struct {
	pb.UnimplementedScheduleServiceServer
	db     types.DB
	sched  *Scheduler
	config config.ServerConfig
}

func NewServer(db types.DB, sched *Scheduler, config config.ServerConfig) *server {
	return &server{db: db, sched: sched, config: config}
}

func (s *server) NewTenant(ctx context.Context, req *pb.NewTenantRequest) (*pb.Response, error) {
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
		if err == types.ErrorTenantAlreadyExists {
			err = status.Error(codes.AlreadyExists, types.ErrorTenantAlreadyExists.Error())
			return nil, err
		}
		return nil, err
	}
	return resp, nil
}

func (s *server) TenantTaskInfo(ctx context.Context, req *pb.TenantTaskRequest) (resp *pb.TenantTaskResponse, err error) {
	queue, ex := s.sched.cs[req.GetTenantID()]
	pendingCount, err := s.db.FindTenantPendingTaskCount(ctx, types.GetTenantPendingTaskOption{TenantId: req.TenantID})
	if err != nil {
		return nil, err
	}
	if ex {
		limit := queue.Tenant.ResourceQuota.Concurrency.Int64
		runningTasks := 0
		runningTasks, err = s.sched.em.CountTasks(req.TenantID)
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
		tenant, err = s.db.FindTenant(ctx, types.GetTenantInfoOption{TenantId: &req.TenantID})
		if err != nil {
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

func (s *server) NewTask(ctx context.Context, req *pb.NewTaskRequest) (*pb.Response, error) {
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
			runningTasks, err := s.sched.em.CountTasks(req.TenantId)
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
	if err := s.db.CreateNewTask(ctx, t); err != nil {
		return nil, err
	}
	resp := &pb.Response{
		Code: pb.Code_Ok,
	}
	return resp, nil
}

func (s *server) PauseTask(ctx context.Context, req *pb.PauseTaskRequest) (*pb.Response, error) {
	panic("implement me")
}

func (s *server) RestartTask(ctx context.Context, req *pb.RestartTaskRequest) (*pb.Response, error) {
	panic("implement me")
}

func (s *server) StopTask(ctx context.Context, req *pb.StopTaskRequest) (*pb.Response, error) {
	panic("implement me")
}

func (s *server) QueryTaskStatus(ctx context.Context, req *pb.QueryTaskRequest) (*pb.QueryStatusResponse, error) {
	options := types.GetTaskStatusOption{Uid: req.Uid}
	taskStatus, err := s.db.GetTaskStatus(ctx, options)
	if err != nil {
		return nil, err
	}
	resp := &pb.QueryStatusResponse{
		Code:   pb.Code_Ok,
		Status: string(taskStatus),
	}
	return resp, nil
}

func (s *server) Start() {
	// Create a listener on TCP port
	lis, err := net.Listen("tcp", s.config.GrpcAddress)
	if err != nil {
		log.Fatal().Msgf("failed to listen: %v", err)
	}

	// Create a gRPC server object
	srv := grpc.NewServer()
	// Attach the Greeter service to the server
	pb.RegisterScheduleServiceServer(srv, s)
	// Serve gRPC server
	log.Info().Msg(fmt.Sprintf("serving gRPC on %s", s.config.GrpcAddress))
	go func() {
		log.Fatal().Err(srv.Serve(lis)).Send()
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
		log.Fatal().Msgf("failed to dial gRPC server: %v", err)
	}

	gwmux := runtime.NewServeMux()
	// Register Greeter
	err = pb.RegisterScheduleServiceHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatal().Msgf("failed to register gateway: %v", err)
	}

	gwServer := &http.Server{
		Addr:    s.config.HttpAddress,
		Handler: gwmux,
	}

	log.Info().Msg(fmt.Sprintf("serving gRPC-Gateway on http://%s", s.config.HttpAddress))
	log.Fatal().Err(gwServer.ListenAndServe()).Send()
}
