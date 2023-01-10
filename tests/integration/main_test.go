package integration

import (
	"context"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/pb"
	"github.com/mykube-run/keel/tests/testkit/scheduler"
	"github.com/mykube-run/keel/tests/testkit/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
	"time"
)

func prepare(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	log.Info().Msg("starting integration test workers and scheduler")
	go worker.StartTestWorkers(t)
	go scheduler.StartScheduler(t)

	time.Sleep(time.Second * 2)
	cfg := config.DefaultFromEnv()
	addr := fmt.Sprintf("%v:%v", cfg.Scheduler.Address, cfg.Scheduler.Port+1000)
	conn, err := grpc.DialContext(context.TODO(), addr, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("unable to connect to scheduler grpc api: %v", err)
	}
	client = pb.NewScheduleServiceClient(conn)
}

var (
	client pb.ScheduleServiceClient
)

const (
	tenantId  = "tenant-integration-test-1"
	zone      = "global"
	partition = "global-scheduler-1"
	taskId    = "task-integration-test-1"
)

func Test_Prepare(t *testing.T) {
	prepare(t)
}

func Test_CreateTenant(t *testing.T) {
	req := &pb.CreateTenantRequest{
		Uid:      tenantId,
		Zone:     zone,
		Priority: 0,
		Name:     "Integration Test Tenant",
		Quota: &pb.ResourceQuota{
			Type:  string(enum.ResourceTypeConcurrency),
			Value: 10,
		},
	}
	resp, err := client.CreateTenant(context.TODO(), req)
	if err != nil {
		t.Fatalf("should be able to create tenant, but got error: %v", err)
	}
	if resp.Code != pb.Code_Ok {
		t.Fatalf("should be able to create tenant, but got code: %v (%v)", resp.Code, resp.Message)
	}
}

func Test_CreateTask(t *testing.T) {
	req := &pb.CreateTaskRequest{
		Options:          nil,
		Type:             string(enum.TaskTypeUserTask),
		TenantId:         tenantId,
		Uid:              taskId + "-unknown",
		Handler:          "unknown",
		Config:           []byte(`{"key": "value"}`),
		ScheduleStrategy: "",
		Priority:         0,
	}
	resp, err := client.CreateTask(context.TODO(), req)
	if err != nil {
		t.Fatalf("should be able to create task, but got error: %v", err)
	}
	if resp.Code != pb.Code_Ok {
		t.Fatalf("should be able to create task, but got code: %v (%v)", resp.Code, resp.Message)
	}
}

func Test_OrdinaryTaskHandler(t *testing.T) {
	// 1. Create task
	{
		req := &pb.CreateTaskRequest{
			Options:          nil,
			Type:             string(enum.TaskTypeUserTask),
			TenantId:         tenantId,
			Uid:              taskId + "-ordinary",
			Handler:          "ordinary",
			Config:           []byte(`{"key": "value"}`),
			ScheduleStrategy: "",
			Priority:         0,
		}
		resp, err := client.CreateTask(context.TODO(), req)
		if err != nil {
			t.Fatalf("should be able to create task, but got error: %v", err)
		}
		if resp.Code != pb.Code_Ok {
			t.Fatalf("should be able to create task, but got code: %v (%v)", resp.Code, resp.Message)
		}
	}

	// 2. Wait for 8 seconds and check task status
	{
		time.Sleep(time.Second * 8)
		req := &pb.QueryTaskStatusRequest{
			Type: string(enum.TaskTypeUserTask),
			Uid:  taskId + "-ordinary",
		}
		resp, err := client.QueryTaskStatus(context.TODO(), req)
		if err != nil {
			t.Fatalf("failed to query task status: %v", err)
		}
		if resp.Code != pb.Code_Ok {
			t.Fatalf("failed to query task status: %v", resp.Code)
		}
		log.Info().Msgf("current task status: %v", resp.Status)
		if resp.Status == string(enum.TaskStatusPending) {
			t.Fatalf("expecting ordinary tasks' status to be %v, but got %v", enum.TaskStatusRunning, resp.Status)
		}
	}

	// 3. Wait for 120 seconds and check task status
	{
		time.Sleep(time.Second * 120)
		req := &pb.QueryTaskStatusRequest{
			Type: string(enum.TaskTypeUserTask),
			Uid:  taskId + "-ordinary",
		}
		resp, err := client.QueryTaskStatus(context.TODO(), req)
		if err != nil {
			t.Fatalf("failed to query task status: %v", err)
		}
		if resp.Code != pb.Code_Ok {
			t.Fatalf("failed to query task status: %v", resp.Code)
		}
		log.Info().Msgf("current task status: %v", resp.Status)
		if resp.Status != string(enum.TaskStatusSuccess) {
			t.Fatalf("expecting ordinary tasks' status to be %v, but got %v", enum.TaskStatusSuccess, resp.Status)
		}
	}
}
