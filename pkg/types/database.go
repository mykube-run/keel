package types

import (
	"context"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

type DB interface {
	// CreateTenant creates a new tenant with the provided details
	CreateTenant(ctx context.Context, t entity.Tenant) error

	// ActivateTenants updates the active time for specified tenants
	ActivateTenants(ctx context.Context, opt ActivateTenantsOption) error

	// GetTenant retrieves a tenant by ID
	GetTenant(ctx context.Context, opt GetTenantOption) (*entity.Tenant, error)

	// FindActiveTenants retrieves active tenants filtered by optional criteria
	FindActiveTenants(ctx context.Context, opt FindActiveTenantsOption) (entity.Tenants, error)

	// CountTenantPendingTasks counts pending tasks for a tenant within a specified time range
	CountTenantPendingTasks(ctx context.Context, opt CountTenantPendingTasksOption) (int64, error)

	//
	// GetTask retrieves a task by its unique identifier
	GetTask(ctx context.Context, opt GetTaskOption) (*entity.Task, error)

	// GetTaskStatus retrieves the status of a specific task
	GetTaskStatus(ctx context.Context, opt GetTaskStatusOption) (enum.TaskStatus, error)

	// FindPendingTasks retrieves tasks that are in pending states, filtered by tenant and other criteria
	FindPendingTasks(ctx context.Context, opt FindPendingTasksOption) (entity.Tasks, error)

	// CreateTask creates a new task with the provided details
	CreateTask(ctx context.Context, t entity.Task) error

	// UpdateTaskStatus updates the status of one or more tasks
	UpdateTaskStatus(ctx context.Context, opt UpdateTaskStatusOption) error

	// Close releases any resources used by the database connection
	Close() error
}

// FindActiveTenantsOption specifies options for finding active tenants
// NOTE: All fields are optional; nil values indicate no filtering for that field
type FindActiveTenantsOption struct {
	From      *time.Time // Start time for filtering active tenants
	Zone      *string    // Zone to filter tenants by
	Partition *string    // Partition to filter tenants by
}

// FindPendingTasksOption specifies options for finding recent tasks
// NOTE: All fields are optional except Status, which must contain at least one status if provided
type FindPendingTasksOption struct {
	TenantId *string           // Tenant ID to filter tasks by
	MinUid   *string           // Minimum UID value for pagination
	Status   []enum.TaskStatus // List of task statuses to include
}

// GetTaskOption specifies the options for getting task by uid
// NOTE: TenantId is required by the Redis DB implementation for proper routing
type GetTaskOption struct {
	TenantId string // Required by Redis DB implementation for sharding
	Uid      string // Unique identifier of the task
}

// UpdateTaskStatusOption specifies the options for updating user tasks' status
// NOTE: TenantId is required by the Redis DB implementation for proper routing
type UpdateTaskStatusOption struct {
	TenantId string          // Required by Redis DB implementation for sharding
	Uids     []string        // List of task IDs to update
	Status   enum.TaskStatus // New status to set for the tasks
}

// GetTaskStatusOption specifies the options for getting user task's status
// NOTE: TenantId is required by the Redis DB implementation for proper routing
type GetTaskStatusOption struct {
	TenantId string // Required by Redis DB implementation for sharding
	Uid      string // Unique identifier of the task
}

// GetTenantOption specifies options for getting tenant entity
type GetTenantOption struct {
	TenantId string // Unique identifier of the tenant
}

// CountTenantPendingTasksOption specifies options for counting the number of pending tasks of specified tenant
// NOTE: TenantId is required, and From/To define the time range for pending tasks.
type CountTenantPendingTasksOption struct {
	TenantId string    // Unique identifier of the tenant
	From     time.Time // Start of the time range
	To       time.Time // End of the time range
}

// ActivateTenantsOption specifies options for updating tenants' active time
type ActivateTenantsOption struct {
	TenantId   []string  // List of tenant IDs to update
	ActiveTime time.Time // New active time to set for the tenants
}
