package mysql

import (
	"context"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"testing"
	"time"
)

var db *MySQL

const (
	dsn       = "root:pa88w0rd@tcp(mysql:3306)/keel?charset=utf8mb4&parseTime=true"
	tenantId  = "tenant-unit-test-1"
	zone      = "global"
	partition = "global-scheduler-1"
	taskId    = "task-unit-test-1"
)

func deleteTestData() {
	_, _ = db.db.Exec(fmt.Sprintf(`DELETE FROM tenant WHERE uid = '%v'`, tenantId))
	_, _ = db.db.Exec(fmt.Sprintf(`DELETE FROM task WHERE uid = '%v'`, taskId))
}

func Test_New(t *testing.T) {
	var err error
	db, err = New(dsn)
	if err != nil {
		t.Fatalf("should be able to connect to database, got erorr: %v", err)
	}
}

func TestMySQL_CreateTenant(t *testing.T) {
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
		TenantId:    tenant.Uid,
		Concurrency: 10,
	}
	tenant.ResourceQuota = quota
	err := db.CreateTenant(context.TODO(), tenant)
	if err != nil {
		t.Fatalf("should create the tenant, but got error: %v", err)
	}

	err = db.CreateTenant(context.TODO(), tenant)
	if err != enum.ErrTenantAlreadyExists {
		t.Fatalf("should get ErrTenantAlreadyExists, but got error: %v", err)
	}
}

func TestMySQL_FindActiveTenants(t *testing.T) {
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

func TestMySQL_GetTenant(t *testing.T) {
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
	if !tenant.ResourceQuota.ConcurrencyEnabled() || tenant.ResourceQuota.Concurrency == 0 {
		t.Fatalf("unexpected tenant resource quota value: %+v", tenant.ResourceQuota.Concurrency)
	}
}

func TestMySQL_ActivateTenants(t *testing.T) {
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

func TestMySQL_CreateTask(t *testing.T) {
	deleteTestData()

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

func TestMySQL_FindPendingTasks(t *testing.T) {
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

func TestMySQL_CountTenantPendingTasks(t *testing.T) {
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

func TestMySQL_GetTask(t *testing.T) {
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

func TestMySQL_GetTaskStatus(t *testing.T) {
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

func TestMySQL_UpdateTaskStatus(t *testing.T) {
	{
		opt := types.UpdateTaskStatusOption{
			Uids:   []string{taskId},
			Status: enum.TaskStatusCanceled,
		}
		err := db.UpdateTaskStatus(context.TODO(), opt)
		if err != nil {
			t.Fatalf("should be able to update task status, but got error: %v", err)
		}
	}
	{
		opt := types.GetTaskStatusOption{
			Uid: taskId,
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

func Test_inPlaceHolders(t *testing.T) {
	type args struct {
		n int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "0 place holders",
			args: args{0},
			want: "",
		},
		{
			name: "1 place holder",
			args: args{1},
			want: "?",
		},
		{
			name: "2 place holders",
			args: args{2},
			want: "?, ?",
		},
		{
			name: "3 place holders",
			args: args{3},
			want: "?, ?, ?",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inPlaceHolders(tt.args.n); got != tt.want {
				t.Errorf("inPlaceHolders() = %v, want %v", got, tt.want)
			}
		})
	}
}
