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

const (
    grpcAddr    = "scheduler:9000"
    tenantGRPC  = "tenant-grpc-static"
    zoneGRPC    = "global"
    partitionGR = "global-scheduler-1"
)

func Test_GRPCTransport_Static_DispatchMultiTasks(t *testing.T) {
    zerolog.SetGlobalLevel(zerolog.InfoLevel)
    conn, err := grpc.DialContext(context.TODO(), grpcAddr, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        t.Fatalf("unable to connect to scheduler grpc api: %v", err)
    }
    client := pb.NewScheduleServiceClient(conn)

    // Ensure tenant exists (idempotent)
    {
        req := &pb.CreateTenantRequest{
            Uid:      tenantGRPC,
            Zone:     zoneGRPC,
            Priority: 0,
            Name:     "GRPC Static Tenant",
            Quota: &pb.ResourceQuota{
                Concurrency: 10,
            },
        }
        resp, err := client.CreateTenant(context.TODO(), req)
        if err != nil {
            t.Fatalf("create tenant error: %v", err)
        }
        if resp.Code != pb.Code_Ok && resp.Code != pb.Code_ResourceAlreadyExists {
            t.Fatalf("create tenant code: %v", resp.Code)
        }
    }

    // Create multiple tasks
    n := 10
    taskIds := make([]string, 0, n)
    for i := 0; i < n; i++ {
        uid := fmt.Sprintf("task-grpc-static-%d", i)
        req := &pb.CreateTaskRequest{
            Options:          nil,
            TenantId:         tenantGRPC,
            Uid:              uid,
            Handler:          "ordinary",
            Config:           `{"key":"value"}`,
            ScheduleStrategy: "",
            Priority:         0,
        }
        resp, err := client.CreateTask(context.TODO(), req)
        if err != nil {
            t.Fatalf("create task error: %v", err)
        }
        if resp.Code != pb.Code_Ok {
            t.Fatalf("create task code: %v", resp.Code)
        }
        taskIds = append(taskIds, uid)
    }

    deadline := time.NewTimer(180 * time.Second)
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    remaining := make(map[string]struct{})
    for _, id := range taskIds {
        remaining[id] = struct{}{}
    }

    for len(remaining) > 0 {
        select {
        case <-deadline.C:
            t.Fatalf("timeout waiting tasks to finish, remaining: %v", len(remaining))
        case <-ticker.C:
            for id := range remaining {
                req := &pb.QueryTaskStatusRequest{TenantId: tenantGRPC, Uid: id}
                resp, err := client.QueryTaskStatus(context.TODO(), req)
                if err != nil {
                    t.Fatalf("query status error: %v", err)
                }
                if resp.Code != pb.Code_Ok {
                    t.Fatalf("query status code: %v", resp.Code)
                }
                log.Info().Msgf("task %s status: %s", id, resp.Status)
                if resp.Status == string(enum.TaskStatusSuccess) {
                    delete(remaining, id)
                }
            }
        }
    }
}