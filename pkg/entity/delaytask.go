package entity

import (
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

// DelayTasks is an array of DelayTask
type DelayTasks []*DelayTask

// DelayTask defines the delay task
type DelayTask struct {
	Id               string
	TenantId         string
	Uid              string
	Handler          string
	Config           interface{}
	ScheduleStrategy string
	Priority         int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
	TimeToRun        time.Time
	Status           enum.TaskStatus
}
