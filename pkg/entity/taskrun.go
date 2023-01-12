package entity

import (
	"time"
)

// TaskRuns is an array of TaskRun
type TaskRuns []*TaskRun

type TaskRun struct {
	Id           string     `json:"id" bson:"_id"`
	TenantId     string     `json:"tenantId" bson:"tenantId"`
	TaskId       string     `json:"taskId" bson:"taskId"`
	TaskType     string     `json:"taskType" bson:"taskType"`
	ScheduleType string     `json:"scheduleType" bson:"scheduleType"`
	Status       string     `json:"status" bson:"status"`
	Result       *string    `json:"result" bson:"result"`
	Error        *string    `json:"error" bson:"error"`
	Start        *time.Time `json:"start" bson:"start"`
	End          *time.Time `json:"end" bson:"end"`
}

type TaskRunLogs []*TaskRunLog

type TaskRunLog struct {
	Id  int64
	Log string
}
