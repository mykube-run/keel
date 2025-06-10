package enum

// TaskMessageType defines task message type
type TaskMessageType string

const (
	RetryTask        TaskMessageType = "RetryTask"
	ReportTaskStatus TaskMessageType = "ReportTaskStatus"
	TaskStarted      TaskMessageType = "TaskStarted"
	TaskFinished     TaskMessageType = "TaskFinished"
	TaskFailed       TaskMessageType = "TaskFailed"
	StartMigration   TaskMessageType = "StartMigration"
	FinishMigration  TaskMessageType = "FinishMigration"
)

type ScheduleTaskMessageType string

const (
	StopTask ScheduleTaskMessageType = "StopTask"
)
