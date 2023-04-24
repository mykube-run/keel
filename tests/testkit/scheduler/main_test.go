package scheduler

import (
	"context"
	"fmt"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/pb"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
	"time"
)

var (
	client pb.ScheduleServiceClient
)

const (
	addr      = "scheduler:9000"
	tenantId  = "tenant-integration-test-1"
	zone      = "global"
	partition = "global-scheduler-1"
	taskId    = "task-integration-test-1"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	conn, err := grpc.DialContext(context.TODO(), addr, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Msgf("unable to connect to scheduler grpc api: %v", err)
	}
	client = pb.NewScheduleServiceClient(conn)
	log.Info().Msgf("initialized integration tests")
}

func Test_CreateTenant(t *testing.T) {
	req := &pb.CreateTenantRequest{
		Uid:      tenantId,
		Zone:     zone,
		Priority: 0,
		Name:     "Integration Test Tenant",
		Quota: &pb.ResourceQuota{
			Concurrency: 10,
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

//func Test_CreateTask(t *testing.T) {
//	req := &pb.CreateTaskRequest{
//		Options:          nil,
//		Type:             string(enum.TaskTypeUserTask),
//		TenantId:         tenantId,
//		Uid:              taskId + "-unknown",
//		Handler:          "unknown",
//		Config:           []byte(`{"key": "value"}`),
//		ScheduleStrategy: "",
//		Priority:         0,
//	}
//	resp, err := client.CreateTask(context.TODO(), req)
//	if err != nil {
//		t.Fatalf("should be able to create task, but got error: %v", err)
//	}
//	if resp.Code != pb.Code_Ok {
//		t.Fatalf("should be able to create task, but got code: %v (%v)", resp.Code, resp.Message)
//	}
//
//	req2 := &pb.StopTaskRequest{
//		Type: string(enum.TaskTypeUserTask),
//		Uid:  taskId + "-unknown",
//	}
//	_, _ = client.StopTask(context.TODO(), req2)
//}

func Test_OrdinaryTaskHandler(t *testing.T) {
	failC := make(chan error)
	finishC := time.NewTimer(time.Second * 200).C

	// 1. Create task
	{
		req := &pb.CreateTaskRequest{
			Options: nil,
			// Type:             string(enum.TaskTypeUserTask),
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

	go func() {
		// 2. Wait for 8 seconds and check task status
		time.Sleep(time.Second * 8)
		req := &pb.QueryTaskStatusRequest{
			// Type: string(enum.TaskTypeUserTask),
			TenantId: tenantId,
			Uid:      taskId + "-ordinary",
		}
		resp, err := client.QueryTaskStatus(context.TODO(), req)
		if err != nil {
			failC <- fmt.Errorf("failed to query task status: %v", err)
		}
		if resp.Code != pb.Code_Ok {
			failC <- fmt.Errorf("failed to query task status: %v", resp.Code)
		}
		log.Info().Msgf("current task status: %v", resp.Status)
		if resp.Status == string(enum.TaskStatusPending) {
			failC <- fmt.Errorf("expecting ordinary tasks' status to be %v, but got %v", enum.TaskStatusRunning, resp.Status)
		}
	}()

	go func() {
		// 3. Wait for 300 seconds and check task status
		time.Sleep(time.Second * 300)
		req := &pb.QueryTaskStatusRequest{
			// Type: string(enum.TaskTypeUserTask),
			TenantId: tenantId,
			Uid:      taskId + "-ordinary",
		}
		resp, err := client.QueryTaskStatus(context.TODO(), req)
		if err != nil {
			failC <- fmt.Errorf("failed to query task status: %v", err)
		}
		if resp.Code != pb.Code_Ok {
			failC <- fmt.Errorf("failed to query task status: %v", resp.Code)
		}
		log.Info().Msgf("current task status: %v", resp.Status)
		if resp.Status != string(enum.TaskStatusSuccess) {
			failC <- fmt.Errorf("expecting ordinary tasks' status to be %v, but got %v", enum.TaskStatusSuccess, resp.Status)
		}
	}()

	select {
	case err := <-failC:
		if err != nil {
			t.Fatal(err)
		}
	case <-finishC:
		return
	}
}
