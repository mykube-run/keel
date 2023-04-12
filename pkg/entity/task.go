package entity

import (
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

// Tasks is an array of Task
type Tasks []Task

// TaskIds returns task ids
func (ut Tasks) TaskIds() []string {
	val := make([]string, 0)
	for _, t := range ut {
		val = append(val, t.Uid)
	}
	return val
}

// Task defines the user task
type Task struct {
	Uid              string          `json:"uid" bson:"uid"`
	TenantId         string          `json:"tenantId" bson:"tenantId"`
	Handler          string          `json:"handler" bson:"handler"`
	Config           interface{}     `json:"config" bson:"config"`
	ScheduleStrategy string          `json:"scheduleStrategy" bson:"scheduleStrategy"`
	Priority         int32           `json:"priority" bson:"priority"`
	Progress         int32           `json:"progress" bson:"progress"`
	CreatedAt        time.Time       `json:"createdAt" bson:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt" bson:"updatedAt"`
	Status           enum.TaskStatus `json:"status" bson:"status"`
}

func (t *Task) Fields() []interface{} {
	return []interface{}{
		&t.Uid, &t.TenantId, &t.Handler, &t.Config, &t.ScheduleStrategy, &t.Priority, &t.Progress, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	}
}
