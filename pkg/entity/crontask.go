package entity

import (
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

// CronTasks is an array of CronTask
type CronTasks []*CronTask

// CronTask defines the cron task
type CronTask struct {
	Uid              string          `json:"uid" bson:"uid"`
	TenantId         string          `json:"tenantId" bson:"tenantId"`
	Handler          string          `json:"handler" bson:"handler"`
	Name             string          `json:"name" bson:"name"`
	Description      string          `json:"description" bson:"description"`
	Cron             string          `json:"cron" bson:"cron"`
	Config           interface{}     `json:"config" bson:"config"`
	ScheduleStrategy string          `json:"scheduleStrategy" bson:"scheduleStrategy"`
	Priority         int32           `json:"priority" bson:"priority"`
	CreatedAt        time.Time       `json:"createdAt" bson:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt" bson:"updatedAt"`
	NextTick         time.Time       `json:"nextTick" bson:"nextTick"`
	Status           enum.TaskStatus `json:"status" bson:"status"`
}
