package entity

import (
	"time"
)

// TaskRuns is an array of TaskRun
type TaskRuns []*TaskRun

type TaskRun struct {
	Id           int64
	TenantId     string
	TaskId       string
	TaskType     string
	ScheduleType string
	Status       string
	Result       *string
	Error        *string
	Start        *time.Time
	End          *time.Time
}

type TaskRunLogs []*TaskRunLog

type TaskRunLog struct {
	Id  int64
	Log string
}
