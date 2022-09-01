package entity

import (
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

// UserTasks is an array of UserTask
type UserTasks []*UserTask

// TaskIds returns task ids
func (ut UserTasks) TaskIds() []string {
	val := make([]string, 0)
	for _, t := range ut {
		val = append(val, t.Uid)
	}
	return val
}

// UserTask defines the user task
type UserTask struct {
	Id               string
	TenantId         string
	Uid              string
	Handler          string
	Config           interface{}
	ScheduleStrategy string
	Priority         int32
	Progress         int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Status           enum.TaskStatus
}
