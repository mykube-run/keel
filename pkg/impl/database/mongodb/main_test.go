package mongodb

import (
	"context"
	"database/sql"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"go.mongodb.org/mongo-driver/bson"
	"testing"
	"time"
)

var db *MongoDB

const (
	dsn       = "mongodb://root:pa88w0rd@mongodb:27017/"
	tenantId  = "tenant-unit-test-1"
	zone      = "global"
	partition = "global-scheduler-1"
	taskId    = "task-unit-test-1"
)

func deleteTestData() {
	ctx := context.Background()
	_, _ = db.tenant.DeleteOne(ctx, bson.M{"uid": tenantId})
	_, _ = db.userTask.DeleteOne(ctx, bson.M{"uid": taskId})
}

func Test_New(t *testing.T) {
	var err error
	db, err = New(dsn)
	if err != nil {
		t.Fatalf("should be able to connect to database, got erorr: %v", err)
	}
}

func TestMongoDB_CreateTenant(t *testing.T) {
	deleteTestData()

	tenant := entity.Tenant{
		Uid:       tenantId,
		Zone:      zone,
		Partition: partition,
		Priority:  0,
		Name:      "Unit Test Tenant",
		Status:    enum.TenantStatusActive,
	}
	quota := entity.ResourceQuota{
		TenantId: tenant.Uid,
		Type:     string(enum.ResourceTypeConcurrency),
		Concurrency: sql.NullInt64{
			Int64: 10,
			Valid: true,
		},
	}
	tenant.ResourceQuota = quota
	err := db.CreateTenant(context.TODO(), tenant)
	if err != nil {
		t.Fatalf("should create the tenant, but got error: %v", err)
	}

	err = db.CreateTenant(context.TODO(), tenant)
	if err != enum.ErrTenantAlreadyExists {
		t.Fatalf("expecting ErrTenantAlreadyExists, but got error: %v", err)
	}
}

func TestMongoDB_FindActiveTenants(t *testing.T) {
	{
		z := zone
		p := partition
		opt := types.FindActiveTenantsOption{
			Zone:      &z,
			Partition: &p,
		}
		tenants, err := db.FindActiveTenants(context.TODO(), opt)
		if err != nil {
			t.Fatalf("should find active tenants, but got error: %v", err)
		}
		if len(tenants) == 0 {
			t.Fatalf("should find at least one active tenants, but got 0")
		}
		if tenants[0].Uid != tenantId {
			t.Fatalf("should find active tenant %v, but got %v", tenantId, tenants[0].Uid)
		}
	}
	{
		z := "not-exist"
		p := "not-exist-scheduler-1"
		opt := types.FindActiveTenantsOption{
			Zone:      &z,
			Partition: &p,
		}
		tenants, err := db.FindActiveTenants(context.TODO(), opt)
		if err != nil {
			t.Fatalf("should find active tenants, but got error: %v", err)
		}
		if len(tenants) > 0 {
			t.Fatalf("should find zero active tenants in not-exist zone, but got %v", len(tenants))
		}
	}
}

func TestMongoDB_GetTenant(t *testing.T) {
	opt := types.GetTenantOption{
		TenantId: tenantId,
	}
	tenant, err := db.GetTenant(context.TODO(), opt)
	if err != nil {
		t.Fatalf("should find specified tenant, but got error: %v", err)
	}
	if tenant.Zone != zone || tenant.Partition != partition {
		t.Fatalf("unexpected tenant zone or partition")
	}
}

func TestMongoDB_ActivateTenants(t *testing.T) {
	opt1 := types.GetTenantOption{
		TenantId: tenantId,
	}
	tenant, err := db.GetTenant(context.TODO(), opt1)
	if err != nil {
		t.Fatalf("should get specified tenant, but got error: %v", err)
	}
	active := tenant.LastActive

	opt2 := types.ActivateTenantsOption{
		TenantId:   []string{tenantId},
		ActiveTime: time.Now().Add(time.Minute),
	}
	err = db.ActivateTenants(context.TODO(), opt2)
	if err != nil {
		t.Fatalf("should be able to activate tenants, but got error: %v", err)
	}

	opt3 := types.GetTenantOption{
		TenantId: tenantId,
	}
	tenant, err = db.GetTenant(context.TODO(), opt3)
	if err != nil {
		t.Fatalf("should get specified tenant, but got error: %v", err)
	}
	if tenant.LastActive == active {
		t.Fatalf("tenant last_active should be updated, but got the same value before updating: %v", tenant.LastActive)
	}
}

func TestMongoDB_CreateTask(t *testing.T) {
	deleteTestData()

	task := entity.UserTask{
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

func TestMongoDB_FindRecentTasks(t *testing.T) {
	tid := tenantId
	opt := types.FindRecentTasksOption{
		TaskType:      enum.TaskTypeUserTask,
		TenantId:      &tid,
		MinUserTaskId: nil,
		Status:        nil,
	}
	tasks, err := db.FindRecentTasks(context.TODO(), opt)
	if err != nil {
		t.Fatalf("should find recent tasks, but got error: %v", err)
	}
	if len(tasks.UserTasks) == 0 {
		t.Fatalf("should find at least one task, but got 0")
	}
	if tasks.UserTasks[0].Uid != taskId {
		t.Fatalf("should find specified task %v, but got %v", taskId, tasks.UserTasks[0].Uid)
	}
}

func TestMongoDB_CountTenantPendingTasks(t *testing.T) {
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

func TestMongoDB_GetTask(t *testing.T) {
	opt := types.GetTaskOption{
		TaskType: enum.TaskTypeUserTask,
		Uid:      taskId,
	}
	tasks, err := db.GetTask(context.TODO(), opt)
	if err != nil {
		t.Fatalf("should be able to get task, but got error: %v", err)
	}
	if len(tasks.UserTasks) != 1 {
		t.Fatalf("should find exactlly one task, but got %v", len(tasks.UserTasks))
	}
	if tasks.UserTasks[0].Uid != taskId {
		t.Fatalf("should find specified task %v, but got %v", taskId, tasks.UserTasks[0].Uid)
	}
}

func TestMongoDB_GetTaskStatus(t *testing.T) {
	opt := types.GetTaskStatusOption{
		TaskType: enum.TaskTypeUserTask,
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

func TestMongoDB_UpdateTaskStatus(t *testing.T) {
	{
		opt := types.UpdateTaskStatusOption{
			TaskType: enum.TaskTypeUserTask,
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
			TaskType: enum.TaskTypeUserTask,
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
