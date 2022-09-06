package database

import (
	"context"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"time"
)

type MockDB struct {
}

func NewMockDB() *MockDB {
	return &MockDB{}
}

func (m *MockDB) CreateTenant(ctx context.Context, t entity.Tenant) error {
	return nil
}

func (m *MockDB) CreateNewTask(ctx context.Context, t entity.UserTask) error {
	return nil
}

func (m *MockDB) FindActiveTenants(ctx context.Context, opt types.FindActiveTenantsOption) (entity.Tenants, error) {
	panic("implement me")
}

func (m *MockDB) FindRecentTasks(ctx context.Context, opt types.FindRecentTasksOption) (entity.Tasks, error) {
	tasks := entity.Tasks{
		CronTasks:  nil,
		UserTasks:  nil,
		DelayTasks: nil,
	}
	var i int64 = 0
	for i = 1; i <= 6; i++ {
		tasks.UserTasks = append(tasks.UserTasks, newUserTask(i))
	}
	return tasks, nil
}

func (m *MockDB) GetTask(ctx context.Context, opt types.GetTaskOption) (entity.Tasks, error) {
	panic("implement me")
}

func (m *MockDB) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	panic("implement me")
}

func (m *MockDB) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	return "", nil
}

func (m *MockDB) Close() error {
	panic("implement me")
}

func newUserTask(i int64) *entity.UserTask {
	return &entity.UserTask{
		Id:        fmt.Sprintf("%v", i),
		TenantId:  "10001",
		Uid:       fmt.Sprintf("mock-user-task-%v", i),
		Handler:   "mock-user-task",
		Priority:  0,
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    enum.TaskStatusPending,
	}
}
