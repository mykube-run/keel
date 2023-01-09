package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"time"
)

type MockDB struct {
	hdl string
}

func NewMockDB() *MockDB {
	return &MockDB{}
}

func (m *MockDB) CreateTenant(ctx context.Context, t entity.Tenant) error {
	return nil
}

func (m *MockDB) CreateTask(ctx context.Context, t entity.UserTask) error {
	return nil
}

func (m *MockDB) GetTenant(ctx context.Context, opt types.GetTenantOption) (*entity.Tenant, error) {
	tenant := &entity.Tenant{
		Uid:        "tenant-1",
		Zone:       "global",
		Priority:   0,
		Name:       "Tenant 1",
		Status:     string(enum.TaskStatusPending),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		LastActive: time.Now(),
		ResourceQuota: entity.ResourceQuota{
			Id:       "tenant-1",
			TenantId: "tenant-1",
			Type:     string(enum.ResourceTypeConcurrency),
			Concurrency: sql.NullInt64{
				Int64: 3,
				Valid: true,
			},
		},
	}
	return tenant, nil
}

func (m *MockDB) CountTenantPendingTasks(ctx context.Context, opt types.CountTenantPendingTasksOption) (int64, error) {
	return 0, nil
}

func (m *MockDB) FindActiveTenants(ctx context.Context, opt types.FindActiveTenantsOption) (entity.Tenants, error) {
	tenant := &entity.Tenant{
		Uid:        "tenant-1",
		Zone:       "global",
		Priority:   0,
		Name:       "Tenant 1",
		Status:     string(enum.TaskStatusPending),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		LastActive: time.Now(),
		ResourceQuota: entity.ResourceQuota{
			Id:       "tenant-1",
			TenantId: "tenant-1",
			Type:     string(enum.ResourceTypeConcurrency),
			Concurrency: sql.NullInt64{
				Int64: 3,
				Valid: true,
			},
		},
	}
	return []*entity.Tenant{tenant}, nil
}

func (m *MockDB) ActivateTenants(ctx context.Context, opt types.ActivateTenantsOption) error {
	return nil
}

func (m *MockDB) FindRecentTasks(ctx context.Context, opt types.FindRecentTasksOption) (entity.Tasks, error) {
	tasks := entity.Tasks{
		CronTasks:  nil,
		UserTasks:  nil,
		DelayTasks: nil,
	}
	var i int64 = 0
	for i = 1; i <= 4; i++ {
		tasks.UserTasks = append(tasks.UserTasks, m.newUserTask(i))
	}
	return tasks, nil
}

func (m *MockDB) GetTask(ctx context.Context, opt types.GetTaskOption) (entity.Tasks, error) {
	return entity.Tasks{}, fmt.Errorf("task not found")
}

func (m *MockDB) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	return nil
}

func (m *MockDB) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	return "", nil
}

func (m *MockDB) Close() error {
	return nil
}

func (m *MockDB) newUserTask(i int64) *entity.UserTask {
	now := time.Now()
	return &entity.UserTask{
		TenantId:  "tenant-1",
		Uid:       fmt.Sprintf("task-%v-%v", m.hdl, now.Unix()),
		Handler:   "mock-test",
		Priority:  0,
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
		Status:    enum.TaskStatusPending,
	}
}
