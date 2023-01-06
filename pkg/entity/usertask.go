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
	Uid              string
	TenantId         string
	Handler          string
	Config           interface{}
	ScheduleStrategy string
	Priority         int32
	Progress         int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Status           enum.TaskStatus
}

func (t *UserTask) Fields() []interface{} {
	return []interface{}{
		&t.Uid, &t.TenantId, &t.Handler, &t.Config, &t.ScheduleStrategy, &t.Priority, &t.Progress, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	}
}
