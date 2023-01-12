package entity

import (
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

// DelayTasks is an array of DelayTask
type DelayTasks []*DelayTask

// DelayTask defines the delay task
type DelayTask struct {
	Uid              string          `json:"uid" bson:"uid"`
	TenantId         string          `json:"tenantId" bson:"tenantId"`
	Handler          string          `json:"handler" bson:"handler"`
	Config           interface{}     `json:"config" bson:"config"`
	ScheduleStrategy string          `json:"scheduleStrategy" bson:"scheduleStrategy"`
	Priority         int32           `json:"priority" bson:"priority"`
	CreatedAt        time.Time       `json:"createdAt" bson:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt" bson:"updatedAt"`
	TimeToRun        time.Time       `json:"timeToRun" bson:"timeToRun"`
	Status           enum.TaskStatus `json:"status" bson:"status"`
}
