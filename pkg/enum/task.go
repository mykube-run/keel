package enum

// TaskType defines task type
type TaskType string

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

// ScheduleStrategy task schedule strategy
type ScheduleStrategy string

const (
	ScheduleStrategyScheduleOnCreate ScheduleStrategy = "ScheduleOnCreate" // Schedule the task immediately after creation unless tenant resource quota exceeded
)
