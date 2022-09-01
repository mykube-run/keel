package entity

import (
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

// CronTasks is an array of CronTask
type CronTasks []*CronTask

// CronTask defines the cron task
type CronTask struct {
	Id               string
	TenantId         string
	Handler          string
	Name             string
	Description      string
	Cron             string
	Config           interface{}
	ScheduleStrategy string
	Priority         int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
	NextTick         time.Time
	Status           enum.TaskStatus
}
