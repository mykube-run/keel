package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
	"testing"
	"time"
)

type testdb struct {
	id int64
}

func (t *testdb) CreateTenant(ctx context.Context, v entity.Tenant) error {
	return nil
}

func (t *testdb) CreateNewTask(ctx context.Context, v entity.UserTask) error {
	return nil
}

func (t *testdb) FindActiveTenants(ctx context.Context, opt types.FindActiveTenantsOption) (entity.Tenants, error) {
	tenant := &entity.Tenant{
		Id:         "1",
		Uid:        "tenant-1",
		Zone:       "CN",
		Priority:   0,
		Name:       "Tenant 1",
		Status:     string(enum.TaskStatusPending),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		LastActive: time.Now(),
		ResourceQuota: entity.ResourceQuota{
			Id:       "1",
			TenantId: "1",
			Type:     0,
			CPU:      sql.NullInt64{},
			Memory:   sql.NullInt64{},
			Storage:  sql.NullInt64{},
			GPU:      sql.NullInt64{},
			Concurrency: sql.NullInt64{
				Int64: 3,
				Valid: true,
			},
			Custom: sql.NullInt64{},
			Peak:   sql.NullFloat64{},
		},
	}
	return []*entity.Tenant{tenant}, nil
}

func (t *testdb) FindRecentTasks(ctx context.Context, opt types.FindRecentTasksOption) (entity.Tasks, error) {
	return t.GetTask(ctx, types.GetTaskOption{})
}

func (t *testdb) GetTask(ctx context.Context, opt types.GetTaskOption) (entity.Tasks, error) {
	task := &entity.UserTask{
		Id:               "1",
		TenantId:         "tenant-1",
		Uid:              fmt.Sprintf("task-%v", t.id),
		Handler:          "test",
		Config:           nil,
		ScheduleStrategy: "0",
		Priority:         0,
		Progress:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Status:           enum.TaskStatusPending,
	}
	tasks := entity.Tasks{
		UserTasks: []*entity.UserTask{task},
	}
	t.id += 1
	return tasks, nil
}

func (t *testdb) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	log.Info().Strs("taskId", opt.Uids).Interface("status", opt.Status).Msg("updating task status")
	return nil
}

func (t *testdb) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	return "", nil
}

func (t *testdb) Close() error {
	return nil
}

func TestKafkaScheduler(t *testing.T) {
	brokers := strings.Split(os.Getenv("KEEL_KAFKA_BROKERS"), ",")
	opt := &Options{
		Name:             "scheduler-1",
		Zone:             "cn",
		ScheduleInterval: 5,
		StaleCheckDelay:  60,
		Snapshot: config.SnapshotConfig{
			Enabled:      false,
			MaxVersions:  10,
			Interval:     time.Second * 60,
			Endpoint:     os.Getenv("KEEL_SNAPSHOT_ENDPOINT"),
			Region:       os.Getenv("KEEL_SNAPSHOT_REGION"),
			Bucket:       os.Getenv("KEEL_SNAPSHOT_BUCKET"),
			AccessKey:    os.Getenv("KEEL_SNAPSHOT_ACCESS_KEY"),
			AccessSecret: os.Getenv("KEEL_SNAPSHOT_ACCESS_SECRET"),
			Secure:       true,
		},
		Transport: config.TransportConfig{
			Type: "kafka",
			Role: string(enum.TransportRoleScheduler),
			Kafka: config.KafkaConfig{
				Brokers: brokers,
				Topics: config.KafkaTopics{
					Tasks:    []string{"cloud-keel-tasks-test-0"},
					Messages: []string{"cloud-keel-messages-test"},
				},
				MessageTTL: 60 * 60,
			},
		},
	}
	db := new(testdb)
	lg := zerolog.New(os.Stdout)
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	s, err := New(opt, db, &lg)
	if err != nil {
		t.Fatalf("error creating scheduler: %v", err)
	}
	s.Start()
}
