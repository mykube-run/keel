package queue

import (
	"container/heap"
	"context"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"sync"
	"time"
)

const FetchFromDBWatermarkRatio = 2

// TaskQueue is in-memory cache for tenant tasks and resource quota, automatically handling
// fetching tasks from database when necessary.
type TaskQueue struct {
	Tenant *entity.Tenant
	Tasks  *PriorityQueue

	// User task id is incremental, hence maxUid is used to
	// avoid populating tasks that already exist in queue
	maxUid string
	mu     sync.RWMutex
	db     types.DB
	lg     types.Logger
	hooks  types.Hooks
}

func NewTaskQueue(db types.DB, lg types.Logger, t *entity.Tenant, hooks types.Hooks) *TaskQueue {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	c := &TaskQueue{
		Tenant: t,
		Tasks:  &pq,
		db:     db,
		lg:     lg,
		hooks:  hooks,
	}
	return c
}

// PopTasks pops at most n entity.Tasks from cache, returns tasks and the actual number successfully popped.
// When the number of cached entity.Tasks is zero or less than FetchFromDBWatermarkRatio * n, it will populate tasks in background.
func (c *TaskQueue) PopTasks(n int) (tasks entity.Tasks, popped int, err error) {
	c.mu.Lock()

	for c.Tasks.Len() > 0 {
		item := heap.Pop(c.Tasks).(*Item)
		tasks = append(tasks, item.Value().(*entity.Task))
		popped += 1
		if popped >= n {
			break
		}
	}

	// Do not forget to unlock
	c.mu.Unlock()

	if c.Tasks.Len() < FetchFromDBWatermarkRatio*n || c.Tasks.Len() == 0 {
		go c.FetchTasks()
	}
	return
}

// EnqueueTask put task back to the priority queue, adding delta to task's priority.
// This is useful when task has to be retried as soon as possible before processing other tasks.
func (c *TaskQueue) EnqueueTask(task *entity.Task, delta int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	task.Priority = task.Priority + int32(delta)
	item := NewItem(int(task.Priority), task)
	heap.Push(c.Tasks, item)
}

// FetchTasks read tasks from database and populate in-memory queue
func (c *TaskQueue) FetchTasks() {
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	if err := c.populateTasks(ctx); err != nil {
		c.lg.Log(types.LevelError, "error", err.Error(), "tenantId", c.Tenant.Uid, "message", "failed to fetch tasks from database")
	}
}

func (c *TaskQueue) populateTasks(ctx context.Context) error {
	var status []enum.TaskStatus
	if c.maxUid == "" {
		status = []enum.TaskStatus{enum.TaskStatusPending, enum.TaskStatusScheduling}
	} else {
		status = []enum.TaskStatus{enum.TaskStatusPending}
	}
	opt := types.FindPendingTasksOption{
		TenantId: &c.Tenant.Uid,
		// MinUid: &c.maxUid,
		MinUid: nil,
		Status: status,
	}
	tasks, err := c.db.FindPendingTasks(ctx, opt)
	if err != nil {
		c.lg.Log(types.LevelTrace, "tenantId", c.Tenant.Uid, "tasks", len(tasks), "message", "fetched tasks from database")
		return err
	}
	if err = c.db.UpdateTaskStatus(ctx, types.UpdateTaskStatusOption{
		TenantId: c.Tenant.Uid,
		Uids:     tasks.TaskIds(),
		Status:   enum.TaskStatusScheduling,
	}); err != nil {
		c.lg.Log(types.LevelTrace, "error", err.Error(), "tenantId", c.Tenant.Uid, "message", "failed to update task status to Scheduling")
		return err
	}

	for _, t := range tasks {
		le := types.HookEvent{Task: types.NewTaskMetadataFromTaskEntity(t)}
		c.hooks.OnTaskScheduling(le)
	}

	n := 0
	for _, v := range tasks {
		vc := v
		c.maxUid = vc.Uid
		item := NewItem(int(vc.Priority), vc)
		heap.Push(c.Tasks, item)
		n += 1
	}
	if n > 0 {
		c.lg.Log(types.LevelTrace, "tenantId", c.Tenant.Uid, "tasks", n, "message", "populating task queue from database")
	}
	return nil
}

func (c *TaskQueue) PopAllTasks() (tasks entity.Tasks, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for c.Tasks.Len() > 0 {
		item := heap.Pop(c.Tasks).(*Item)
		tasks = append(tasks, item.Value().(*entity.Task))
	}
	return
}
