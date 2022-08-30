package enum

// TaskMessageType defines task message type
type TaskMessageType string

const (
	RetryTask        TaskMessageType = "RetryTask"
	ReportTaskStatus TaskMessageType = "ReportTaskStatus"
	TaskStarted      TaskMessageType = "TaskStarted"
	TaskFinished     TaskMessageType = "TaskFinished"
	TaskFailed       TaskMessageType = "TaskFailed"
	StartTransition  TaskMessageType = "StartTransition"
	FinishTransition TaskMessageType = "FinishTransition"
)
