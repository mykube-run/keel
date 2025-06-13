package base_redis

import (
	"context"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/redis/go-redis/v9"
	"os"
	"testing"
	"time"
)

var (
	db *Redis
)

const (
	//dsn      = "redis://localhost:7379"
	dsn      = "redis://pa88w0rd@redis:6379"
	tenantId = "tenant-unit-test-1"
	taskId   = "task-unit-test-1"
)

func deleteTestData() {
	var deleteScript = redis.NewScript(common + `
local function delete_test_data(tenant_id, uid)
	local buckets_key = tenant_buckets_key(tenant_id)
    local buckets = redis.call('ZRANGEBYSCORE', buckets_key, 0, '+inf', 'LIMIT', 0, limit)
	
	for idx, value in ipairs(buckets) do
        local bucket = bucket_key(tenant_id, value)
        redis.call('DEL', bucket)
		redis.log(log_level, string.format('[DeleteTestData] deleted %s', bucket))
    end
	redis.call('DEL', buckets_key)
	redis.call('DEL', task_key(tenant_id, uid))
	redis.log(log_level, string.format('[DeleteTestData] deleted %s, %s', buckets_key, task_key(tenant_id, uid)))
	return 0
end

return delete_test_data(KEYS[1], ARGV[1])
`)
	_ = deleteScript.Eval(context.Background(), db.s, []string{tenantId}, taskId).Err()
}

func TestMain(m *testing.M) {
	var err error
	db, err = New(dsn)
	if err != nil {
		fmt.Printf("error creating simple redis: %v\n", err)
		os.Exit(1)
	}

	deleteTestData()
	code := m.Run()
	_ = db.Close()
	os.Exit(code)
}

func TestRedis_CreateTask(t *testing.T) {
	task := entity.Task{
		Uid:              taskId,
		TenantId:         tenantId,
		Handler:          "unit-test",
		Config:           "{\"key\":\"value\"}",
		ScheduleStrategy: "FailFast",
		Priority:         0,
		Progress:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Status:           enum.TaskStatusPending,
	}
	err := db.CreateTask(context.TODO(), task)
	if err != nil {
		t.Fatalf("should be able to create the task, but got error: %v", err)
	}
}

func TestRedis_FindPendingTasks(t *testing.T) {
	tid := tenantId
	opt := types.FindPendingTasksOption{
		TenantId: &tid,
		MinUid:   nil,
		Status:   nil,
	}
	tasks, err := db.FindPendingTasks(context.TODO(), opt)
	if err != nil {
		t.Fatalf("should find recent tasks, but got error: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatalf("should find at least one task, but got 0")
	}
	if tasks[0].Uid != taskId {
		t.Fatalf("should find specified task %v, but got %v", taskId, tasks[0].Uid)
	}
}

func TestRedis_CountTenantPendingTasks(t *testing.T) {
	opt := types.CountTenantPendingTasksOption{
		TenantId: tenantId,
		From:     time.Now().AddDate(0, 0, -1),
		To:       time.Now(),
	}
	cnt, err := db.CountTenantPendingTasks(context.TODO(), opt)
	if err != nil {
		t.Fatalf("should be able to count pending tasks, but got erorr: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("should find 1 pending task, but got %v", cnt)
	}
}

func TestRedis_GetTask(t *testing.T) {
	opt := types.GetTaskOption{
		TenantId: tenantId,
		Uid:      taskId,
	}
	task, err := db.GetTask(context.TODO(), opt)
	if err != nil {
		t.Fatalf("should be able to get task, but got error: %v", err)
	}
	if task == nil {
		t.Fatalf("should find exactlly one task, but got %v", task)
	}
	if task.Uid != taskId {
		t.Fatalf("should find specified task %v, but got %v", taskId, task.Uid)
	}
}

func TestRedis_GetTaskStatus(t *testing.T) {
	opt := types.GetTaskStatusOption{
		TenantId: tenantId,
		Uid:      taskId,
	}
	status, err := db.GetTaskStatus(context.TODO(), opt)
	if err != nil {
		t.Fatalf("should be able to get task, but got error: %v", err)
	}
	if status != enum.TaskStatusPending {
		t.Fatalf("task status should be %v, but got %v", enum.TaskStatusPending, status)
	}
}

func TestRedis_UpdateTaskStatus(t *testing.T) {
	{
		opt := types.UpdateTaskStatusOption{
			TenantId: tenantId,
			Uids:     []string{taskId},
			Status:   enum.TaskStatusCanceled,
		}
		err := db.UpdateTaskStatus(context.TODO(), opt)
		if err != nil {
			t.Fatalf("should be able to update task status, but got error: %v", err)
		}
	}
	{
		opt := types.GetTaskStatusOption{
			TenantId: tenantId,
			Uid:      taskId,
		}
		status, err := db.GetTaskStatus(context.TODO(), opt)
		if err != nil {
			t.Fatalf("should be able to get task, but got error: %v", err)
		}
		if status != enum.TaskStatusCanceled {
			t.Fatalf("task status should be updated to %v, but got %v", enum.TaskStatusCanceled, status)
		}
	}
}
