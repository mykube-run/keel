package queue

import (
	"container/heap"
	"context"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/logger"
	"github.com/mykube-run/keel/pkg/types"
	"sync"
	"time"
)

const FetchFromDBWatermarkRatio = 2

// TaskQueue is in-memory cache for tenant tasks and resource quota, automatically handling
// fetching tasks from database when necessary.
type TaskQueue struct {
	Tenant     *entity.Tenant
	DelayTasks interface{} // TODO: time wheel
	CronTasks  interface{} // TODO: time wheel
	UserTasks  *PriorityQueue

	// User task id is incremental, hence maxUserTaskId is used to
	// avoid populating tasks that already exist in queue
	maxUserTaskId string
	listener      types.Listener
	db            types.DB
	mu            sync.RWMutex
	lg            logger.Logger
}

func NewTaskQueue(db types.DB, lg logger.Logger, t *entity.Tenant, listener types.Listener) *TaskQueue {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	c := &TaskQueue{
		Tenant:    t,
		UserTasks: &pq,
		db:        db,
		listener:  listener,
		lg:        lg,
	}
	return c
}

// PopUserTasks pops at most n entity.UserTasks from cache, returns tasks and the actual number successfully popped.
// When the number of cached entity.UserTasks is zero or less than FetchFromDBWatermarkRatio * n, it will populate tasks in background.
func (c *TaskQueue) PopUserTasks(n int) (tasks entity.UserTasks, popped int, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for c.UserTasks.Len() > 0 {
		item := heap.Pop(c.UserTasks).(*Item)
		tasks = append(tasks, item.Value().(*entity.UserTask))
		popped += 1
		if popped >= n {
			break
		}
	}

	if c.UserTasks.Len() < FetchFromDBWatermarkRatio*n || c.UserTasks.Len() == 0 {
		go c.FetchTasks()
	}
	return
}

// EnqueueUserTask put task back to the priority queue, adding delta to task's priority.
// This is useful when task has to be retried as soon as possible before processing other tasks.
func (c *TaskQueue) EnqueueUserTask(task *entity.UserTask, delta int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	task.Priority = task.Priority + int32(delta)
	item := NewItem(int(task.Priority), task)
	heap.Push(c.UserTasks, item)
}

// FetchTasks read tasks from database and populate in-memory queue
func (c *TaskQueue) FetchTasks() {
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := c.populateTasks(ctx); err != nil {
		_ = c.lg.Log(logger.LevelError, "err", err.Error(), "message", "failed to fetch tasks from database")
	}
}

func (c *TaskQueue) populateTasks(ctx context.Context) error {
	var status []enum.TaskStatus
	if c.maxUserTaskId == "" {
		status = []enum.TaskStatus{enum.TaskStatusPending, enum.TaskStatusScheduling}
	} else {
		status = []enum.TaskStatus{enum.TaskStatusPending}
	}
	opt := types.FindRecentTasksOption{
		TenantId:      &c.Tenant.Uid,
		MinUserTaskId: &c.maxUserTaskId,
		TaskType:      enum.TaskTypeUserTask,
		Status:        status,
	}
	tasks, err := c.db.FindRecentTasks(ctx, opt)
	if err != nil {
		return err
	}
	if err = c.db.UpdateTaskStatus(ctx, types.UpdateTaskStatusOption{
		TaskType: enum.TaskTypeUserTask,
		Uids:     tasks.UserTasks.TaskIds(),
		Status:   enum.TaskStatusScheduling,
	}); err != nil {
		return err
	}
	for _, taskId := range tasks.UserTasks.TaskIds() {
		msg := types.ListenerEventMessage{TenantUID: c.Tenant.Uid, TaskUID: taskId}
		c.listener.OnTaskScheduling(msg)
	}
	n := 0
	for _, v := range tasks.UserTasks {
		vc := v
		c.maxUserTaskId = vc.Id
		item := NewItem(int(vc.Priority), vc)
		heap.Push(c.UserTasks, item)
		n += 1
	}
	_ = c.lg.Log(logger.LevelDebug, "populated", n, "message", "fetched user tasks from database")
	return nil
}
