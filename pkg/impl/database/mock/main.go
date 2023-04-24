package mock

import (
	"context"
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

func (m *MockDB) CreateTask(ctx context.Context, t entity.Task) error {
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
			TenantId:    "tenant-1",
			Concurrency: 3,
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
			TenantId:    "tenant-1",
			Concurrency: 3,
		},
	}
	return []*entity.Tenant{tenant}, nil
}

func (m *MockDB) ActivateTenants(ctx context.Context, opt types.ActivateTenantsOption) error {
	return nil
}

func (m *MockDB) FindPendingTasks(ctx context.Context, opt types.FindPendingTasksOption) (entity.Tasks, error) {
	var (
		i     int64 = 0
		tasks       = make(entity.Tasks, 0)
	)
	for i = 1; i <= 4; i++ {
		tasks = append(tasks, m.newTask(i))
	}
	return tasks, nil
}

func (m *MockDB) GetTask(ctx context.Context, opt types.GetTaskOption) (*entity.Task, error) {
	return nil, fmt.Errorf("task not found")
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

func (m *MockDB) newTask(i int64) *entity.Task {
	now := time.Now()
	return &entity.Task{
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
