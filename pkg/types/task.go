package types

import (
	"encoding/json"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/rs/zerolog/log"
	"time"
)

// TaskContext contains worker info, task details
type TaskContext struct {
	Worker *WorkerInfo `json:"worker"`
	Task   Task        `json:"task"`
}

func (c *TaskContext) NewMessage(typ enum.TaskMessageType, val interface{}) *TaskMessage {
	switch typ {
	case enum.ReportTaskStatus, enum.StartMigration:
		byt, err := json.Marshal(val)
		if err != nil {
			log.Err(err).Str("taskId", c.Task.Uid).Interface("value", val).
				Msg("failed to marshal task message value")
		}
		return &TaskMessage{
			Type:        typ,
			SchedulerId: c.Task.SchedulerId,
			WorkerId:    c.Worker.Id,
			Task:        c.Task,
			Timestamp:   time.Now(),
			Value:       byt,
		}
	default:
		return &TaskMessage{
			Type:        typ,
			SchedulerId: c.Task.SchedulerId,
			WorkerId:    c.Worker.Id,
			Task:        c.Task,
			Timestamp:   time.Now(),
			Value:       nil,
		}
	}
}

func (c *TaskContext) MarkRunning() {
	if c.Task.LastRun == nil {
		c.Task.LastRun = &TaskRun{}
	}
	c.Task.LastRun.Start = time.Now()
}

func (c *TaskContext) MarkDone(status enum.TaskRunStatus, res string, err error) {
	if c.Task.LastRun == nil {
		c.Task.LastRun = &TaskRun{}
	}
	c.Task.LastRun.Status = status
	c.Task.LastRun.Result = res
	c.Task.LastRun.End = time.Now()
	if err != nil {
		c.Task.LastRun.Error = err.Error()
	}
}

func (c *TaskContext) MarkProgress(v int) {
	if c.Task.LastRun == nil {
		c.Task.LastRun = &TaskRun{}
	}
	c.Task.LastRun.Progress = &v
}

// WorkerInfo contains essential info about the worker that tasks need.
type WorkerInfo struct {
	Id         string `json:"id"`         // Worker id
	Generation int64  `json:"generation"` // Worker generation
}

// Task details
type Task struct {
	Handler      string   `json:"handler"`      // Task handler name
	TenantId     string   `json:"tenantId"`     // Task tenant id
	Uid          string   `json:"uid"`          // Task uid, aka taskId
	SchedulerId  string   `json:"schedulerId"`  // Scheduler id that is responsible for scheduling the task
	Config       string   `json:"config"`       // Task configurations in JSON, might be nil
	RestartTimes int      `json:"restartTimes"` // Number of restart
	InTransition bool     `json:"inTransition"` // Indicates whether this task run is a migration task
	LastRun      *TaskRun `json:"lastRun"`      // The last TaskRun, nil when Task is never run
}

// TaskRun task last run info
type TaskRun struct {
	Status   enum.TaskRunStatus `json:"status"`   // Task run status
	Result   string             `json:"result"`   // Task run result
	Error    string             `json:"error"`    // Error when task failed
	Start    time.Time          `json:"start"`    // Time the task began
	End      time.Time          `json:"end"`      // Time the task ended
	Progress *int               `json:"progress"` // Task progress in the range of (0, 100]
}

// TaskStatus is reported everytime HeartBeat method is called.
type TaskStatus struct {
	State         enum.TaskStatus `json:"state"`
	Progress      int             `json:"progress"`
	Error         error           `json:"error"`
	Timestamp     time.Time       `json:"timestamp"`
	ResourceUsage struct {
		CPU         int `json:"cpu"`
		Memory      int `json:"memory"`
		Storage     int `json:"storage"`
		GPU         int `json:"gpu"`
		Concurrency int `json:"concurrency"`
		Custom      int `json:"custom"`
	} `json:"resourceUsage"`
}

// TaskMetadata metadata
type TaskMetadata struct {
	Handler  string `json:"handler"`  // Task handler name
	TenantId string `json:"tenantId"` // Task tenant id
	Uid      string `json:"uid"`      // Task uid, aka taskId
}

func NewTaskMetadataFromTaskEntity(t *entity.Task) TaskMetadata {
	m := TaskMetadata{
		Handler:  t.Handler,
		TenantId: t.TenantId,
		Uid:      t.Uid,
	}
	return m
}

func NewTaskMetadataFromTaskMessage(msg *TaskMessage) TaskMetadata {
	m := TaskMetadata{
		Handler:  msg.Task.Handler,
		TenantId: msg.Task.TenantId,
		Uid:      msg.Task.Uid,
	}
	return m
}
