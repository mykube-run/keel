package enum

// TaskType defines task type
type TaskType string

const (
	TaskTypeCronTask  TaskType = "CronTask"
	TaskTypeDelayTask TaskType = "DelayTask"
	TaskTypeUserTask  TaskType = "UserTask"
	TaskTypeAll       TaskType = "All"
)

// TaskStatus defines task status
type TaskStatus string

const (
	TaskStatusPending      TaskStatus = "Pending"
	TaskStatusScheduling   TaskStatus = "Scheduling"
	TaskStatusDispatched   TaskStatus = "Dispatched"
	TaskStatusRunning      TaskStatus = "Running"
	TaskStatusNeedsRetry   TaskStatus = "NeedsRetry"
	TaskStatusInTransition TaskStatus = "InTransition"
	TaskStatusSuccess      TaskStatus = "Success"
	TaskStatusFailed       TaskStatus = "Failed"
	TaskStatusCanceled     TaskStatus = "Canceled"
)

// TaskRunStatus task run status
type TaskRunStatus string

const (
	TaskRunStatusSucceed TaskRunStatus = "Succeed"
	TaskRunStatusFailed  TaskRunStatus = "Failed"
)
