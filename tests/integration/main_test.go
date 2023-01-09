package integration

import (
	"context"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/pb"
	"github.com/mykube-run/keel/tests/testkit/scheduler"
	"github.com/mykube-run/keel/tests/testkit/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sync"
	"testing"
)

func start() {
	once := sync.Once{}
	once.Do(func() {
		log.Info().Msgf("starting integration test workers and scheduler")
		go worker.StartTestWorkers()
		go scheduler.StartScheduler()
	})
}

const (
	tenantId  = "tenant-integration-test-1"
	zone      = "global"
	partition = "global-scheduler-1"
	taskId    = "task-integration-test-1"
)

func Test_CreateTenant(t *testing.T) {
	start()

	cfg := config.DefaultFromEnv()
	addr := fmt.Sprintf("%v:%v", cfg.Scheduler.Address, cfg.Scheduler.Port+1000)
	conn, err := grpc.DialContext(context.TODO(), addr, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("unable to connect to scheduler grpc api: %v", err)
	}
	client := pb.NewScheduleServiceClient(conn)
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
