package types

import (
	"context"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

type DB interface {
	CreateTenant(ctx context.Context, t entity.Tenant) error
	CreateNewTask(ctx context.Context, t entity.UserTask) error
	FindActiveTenants(context.Context, FindActiveTenantsOption) (entity.Tenants, error)
	FindRecentTasks(context.Context, FindRecentTasksOption) (entity.Tasks, error)
	GetTask(context.Context, GetTaskOption) (entity.Tasks, error)
	UpdateTaskStatus(ctx context.Context, opt UpdateTaskStatusOption) error
	GetTaskStatus(ctx context.Context, opt GetTaskStatusOption) (enum.TaskStatus, error)
	Close() error
}

// FindActiveTenantsOption specifies options for finding active tenants
type FindActiveTenantsOption struct {
	From *time.Time
	Zone *string
}

// FindRecentTasksOption specifies options for finding recent tasks
type FindRecentTasksOption struct {
	TaskType      enum.TaskType
	TenantId      *string
	From          *time.Time
	MinUserTaskId *string
}

// GetTaskOption specifies the options for getting task by uid
type GetTaskOption struct {
	TaskType enum.TaskType
	Uid      string
}

// UpdateTaskStatusOption specifies the options for updating task status
type UpdateTaskStatusOption struct {
	TaskType enum.TaskType
	Uids     []string
	Status   enum.TaskStatus
}

// GetTaskStatusOption specifies the options for finding tasks' status
type GetTaskStatusOption struct {
	TaskType enum.TaskType
	Uid      string
}
