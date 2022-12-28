package types

import (
	"context"
	"errors"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

var ErrorTenantAlreadyExists = errors.New("tenant already exists")

type DB interface {
	CreateTenant(ctx context.Context, t entity.Tenant) error
	CreateNewTask(ctx context.Context, t entity.UserTask) error
	FindActiveTenants(context.Context, FindActiveTenantsOption) (entity.Tenants, error)
	FindRecentTasks(context.Context, FindRecentTasksOption) (entity.Tasks, error)
	GetTask(context.Context, GetTaskOption) (entity.Tasks, error)
	UpdateTaskStatus(ctx context.Context, opt UpdateTaskStatusOption) error
	GetTaskStatus(ctx context.Context, opt GetTaskStatusOption) (enum.TaskStatus, error)
	FindTenant(ctx context.Context, opt GetTenantInfoOption) (entity.Tenant, error)
	FindTenantPendingTaskCount(ctx context.Context, opt GetTenantPendingTaskOption) (int64, error)
	ActiveTenant(ctx context.Context, opt ActiveTenantOption) error // TODO: rename to ActivateTenants
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
	MinUserTaskId *string
	Status        []enum.TaskStatus
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

type GetTenantInfoOption struct {
	TenantId *string
}

type GetTenantPendingTaskOption struct {
	TenantId string
	StartAt  time.Time
	EndAt    time.Time
}

type ActiveTenantOption struct {
	TenantId   []string
	ActiveTime time.Time
}
