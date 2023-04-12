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

	GetTask(ctx context.Context, opt GetTaskOption) (*entity.Task, error)
	GetTaskStatus(ctx context.Context, opt GetTaskStatusOption) (enum.TaskStatus, error)
	FindPendingTasks(ctx context.Context, opt FindPendingTasksOption) (entity.Tasks, error)
	CreateTask(ctx context.Context, t entity.Task) error
	UpdateTaskStatus(ctx context.Context, opt UpdateTaskStatusOption) error

	Close() error
}

// FindActiveTenantsOption specifies options for finding active tenants
type FindActiveTenantsOption struct {
	From      *time.Time
	Zone      *string
	Partition *string
}

// FindPendingTasksOption specifies options for finding recent tasks
type FindPendingTasksOption struct {
	TenantId *string
	MinUid   *string
	Status   []enum.TaskStatus
}

// GetTaskOption specifies the options for getting task by uid
type GetTaskOption struct {
	Uid string
}

// UpdateTaskStatusOption specifies the options for updating user tasks' status
type UpdateTaskStatusOption struct {
	Uids   []string
	Status enum.TaskStatus
}

// GetTaskStatusOption specifies the options for getting user task's status
type GetTaskStatusOption struct {
	Uid string
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
