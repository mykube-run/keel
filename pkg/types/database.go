package types

import (
	"context"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

type DB interface {
	CreateTenant(ctx context.Context, t entity.Tenant) error
	ActivateTenants(ctx context.Context, opt ActivateTenantsOption) error
	GetTenant(ctx context.Context, opt GetTenantOption) (*entity.Tenant, error)
	FindActiveTenants(ctx context.Context, opt FindActiveTenantsOption) (entity.Tenants, error)
	CountTenantPendingTasks(ctx context.Context, opt CountTenantPendingTasksOption) (int64, error)

	GetTask(ctx context.Context, opt GetTaskOption) (entity.Tasks, error)
	GetTaskStatus(ctx context.Context, opt GetTaskStatusOption) (enum.TaskStatus, error)
	FindRecentTasks(ctx context.Context, opt FindRecentTasksOption) (entity.Tasks, error)
	CreateTask(ctx context.Context, t entity.UserTask) error
	UpdateTaskStatus(ctx context.Context, opt UpdateTaskStatusOption) error

	Close() error
}

// FindActiveTenantsOption specifies options for finding active tenants
type FindActiveTenantsOption struct {
	From      *time.Time
	Zone      *string
	Partition *string
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

// GetTenantOption specifies options for getting tenant entity
type GetTenantOption struct {
	TenantId string
}

// CountTenantPendingTasksOption specifies options for counting the number of pending tasks of specified tenant
type CountTenantPendingTasksOption struct {
	TenantId string
	From     time.Time
	To       time.Time
}

// ActivateTenantsOption specifies options for updating tenants' active time
type ActivateTenantsOption struct {
	TenantId   []string
	ActiveTime time.Time
}
