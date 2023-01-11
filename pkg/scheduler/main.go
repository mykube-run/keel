package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/listener"
	"github.com/mykube-run/keel/pkg/impl/transport"
	"github.com/mykube-run/keel/pkg/queue"
	"github.com/mykube-run/keel/pkg/types"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Options struct {
	Name                    string                 // Scheduler name, also used as partition name
	Zone                    string                 // Zone name
	ScheduleInterval        int64                  // Interval in seconds for checking active tenants & new tasks
	StaleCheckDelay         int64                  // Time in seconds for checking stale tasks
	TaskEventUpdateDeadline int64                  // Deadline in seconds for the scheduler to receive task update events
	Snapshot                config.SnapshotConfig  // Scheduler state snapshot configurations
	Transport               config.TransportConfig // Transport config
	ServerConfig            config.ServerConfig    // http and grpc config
}

type Scheduler struct {
	opt    *Options
	mu     sync.Mutex
	cs     map[string]*queue.TaskQueue
	em     *EventManager
	db     types.DB
	lg     types.Logger
	ls     types.Listener
	tran   types.Transport
	srv    *Server
	closeC chan struct{}
}

func New(opt *Options, db types.DB, lg types.Logger, ls types.Listener) (s *Scheduler, err error) {
	if ls == nil {
		ls = listener.Default
	}
	s = &Scheduler{
		opt:    opt,
		cs:     make(map[string]*queue.TaskQueue),
		db:     db,
		lg:     lg,
		ls:     ls,
		closeC: make(chan struct{}),
	}
	s.srv = NewServer(db, s, opt.ServerConfig, lg, ls)
	s.em, err = NewEventManager(opt.Snapshot, s.SchedulerId(), lg)
	if err != nil {
		return nil, err
	}

	if opt.Transport.Type == "kafka" {
		opt.Transport.Kafka.GroupId = s.SchedulerId() /* Update group id */
	}
	s.tran, err = transport.New(&opt.Transport)
	if err != nil {
		return nil, err
	}
	s.tran.OnReceive(s.onReceiveMessage)
	return s, nil
}

func (s *Scheduler) Start() {
	defer func() {
		if r := recover(); r != nil {
			s.printStack(r)
		}
	}()

	s.lg.Log(types.LevelInfo, "schedulerId", s.SchedulerId(), "transport", s.opt.Transport.Type, "message", "starting scheduler")

	if err := s.tran.Start(); err != nil {
		s.lg.Log(types.LevelFatal, "error", err.Error(), "message", "failed to start transport")
	}
	_, _ = s.updateActiveTenants()

	// Start background goroutines
	go s.schedule()
	go s.checkStaleTasks()
	go s.srv.Start()

	stopC := make(chan os.Signal)
	signal.Notify(stopC, os.Interrupt, syscall.SIGTERM /* SIGTERM is expected inside k8s */)

	select {
	case <-stopC:
		s.lg.Log(types.LevelInfo, "message", "received stop signal")
		// close scheduler
		s.closeC <- struct{}{}
		s.resetSchedulingTask()
		key, err := s.em.Backup()
		if err != nil {
			s.lg.Log(types.LevelError, "key", key, "error", err.Error(), "message", "error saving events db snapshot")
		}
	}
}

func (s *Scheduler) schedule() {
	tick := time.NewTicker(time.Duration(s.opt.ScheduleInterval) * time.Second)
	for {
		select {
		case <-s.closeC:
			return
		case <-tick.C:
			if _, err := s.updateActiveTenants(); err != nil {
				s.lg.Log(types.LevelError, "error", err.Error(), "message", "failed to update active tenants")
			}
			for k, c := range s.cs {
				running, err := s.em.CountRunningTasks(k)
				if err != nil {
					s.lg.Log(types.LevelError, "error", err.Error(), "message", "failed to count running tasks")
					continue
				}
				if !c.Tenant.ResourceQuota.Concurrency.Valid {
					s.lg.Log(types.LevelWarn, "message", "tenant resource quota concurrency was invalid")
					continue
				}
				s.lg.Log(types.LevelTrace, "tenantId", k, "quota", c.Tenant.ResourceQuota.Concurrency.Int64,
					"running", running, "message", "tenant concurrency state")
				n := int(c.Tenant.ResourceQuota.Concurrency.Int64) - running
				if n <= 0 {
					continue
				}
				tasks, _, err := c.PopUserTasks(n)
				if err != nil {
					s.lg.Log(types.LevelError, "error", err.Error(), "message", "failed to pop user tasks from local task queue")
					continue
				}
				s.dispatch(tasks)
			}
		}
	}
}

func (s *Scheduler) onReceiveMessage(from, to string, msg []byte) ([]byte, error) {
	if to != s.SchedulerId() {
		return nil, nil
	}

	var m types.TaskMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		s.lg.Log(types.LevelError, "error", err.Error(), "raw", msg, "message", "failed to unmarshal task message")
		return nil, err
	}

	s.handleTaskMessage(&m)
	return nil, nil
}

func (s *Scheduler) handleTaskMessage(m *types.TaskMessage) {
	if m == nil {
		return
	}

	le := types.ListenerEvent{SchedulerId: s.SchedulerId(), WorkerId: m.WorkerId, Task: types.NewTaskMetadataFromTaskMessage(m)}
	ev := NewEventFromMessage(m)
	if err := s.em.Insert(ev); err != nil {
		s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
			"message", "failed to insert task event")
	}

	switch m.Type {
	case enum.RetryTask:
		s.lg.Log(types.LevelWarn, "tenantId", ev.TenantId, "taskId", ev.TaskId, "workerId", ev.WorkerId,
			"detail", ev.Value, "message", "task needs retry")
		s.ls.OnTaskNeedsRetry(le)

		_, ok := s.cs[ev.TenantId]
		if !ok {
			s.lg.Log(types.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId,
				"message", "received message from a tenant not managed by the scheduler")
			return
		}
		s.dispatch([]*entity.UserTask{{TenantId: m.Task.TenantId, Uid: m.Task.Uid,
			Handler: m.Task.Handler, Config: m.Task.Config},
		})
	case enum.TaskFailed:
		s.lg.Log(types.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "workerId", ev.WorkerId,
			"detail", ev.Value, "message", "task run failed")

		if err := s.updateTaskStatus(ev); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
				"message", "failed to update task status")
		}
		if err := s.em.Delete(ev.TenantId, ev.TaskId); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
				"message", "failed to delete task events")
		}
	case enum.ReportTaskStatus:
		// Do Nothing
		return
	case enum.StartTransition:
		s.lg.Log(types.LevelWarn, "tenantId", ev.TenantId, "taskId", ev.TaskId, "workerId", ev.WorkerId,
			"detail", ev.Value, "message", "task transition start")

		// mark task transition and wait worker to finish transition
		if err := s.updateTaskStatus(ev); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
				"message", "failed to update tasks status")
		}
		s.dispatch([]*entity.UserTask{{
			TenantId: m.Task.TenantId, Uid: m.Task.Uid,
			Handler: m.Task.Handler, Config: m.Task.Config},
		})
	case enum.FinishTransition:
		s.lg.Log(types.LevelWarn, "tenantId", ev.TenantId, "taskId", ev.TaskId, "workerId", ev.WorkerId,
			"detail", ev.Value, "message", "task transition finished")

		// mark task running
		if err := s.updateTaskStatus(ev); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
				"message", "failed to update task status")
		}
	case enum.TaskStarted:
		s.lg.Log(types.LevelInfo, "tenantId", ev.TenantId, "taskId", ev.TaskId, "workerId", ev.WorkerId,
			"message", "worker starting to process task")

		if err := s.updateTaskStatus(ev); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
				"message", "failed to update task status")
		}
		s.ls.OnTaskRunning(le)
	case enum.TaskFinished:
		s.lg.Log(types.LevelInfo, "tenantId", ev.TenantId, "taskId", ev.TaskId, "workerId", ev.WorkerId,
			"message", "worker has finished processing task")

		if err := s.updateTaskStatus(ev); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
				"message", "failed to update task status")
		}
		s.ls.OnTaskFinished(le)

		if err := s.em.Delete(ev.TenantId, ev.TaskId); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
				"message", "failed to delete task events")
		}
		runningCount, _ := s.em.CountRunningTasks(ev.TenantId)
		s.lg.Log(types.LevelDebug, "tenantId", ev.TenantId, "running", runningCount,
			"message", "running tasks count")
	}
}

func (s *Scheduler) taskHistory(tenantId, taskId string) (*types.TaskRun, int, bool, error) {
	var (
		retried      = 0
		start        time.Time
		inTransition = false
	)

	err := s.em.Iterate(tenantId, taskId, func(e *TaskEvent) bool {
		switch e.EventType {
		case string(enum.RetryTask):
			retried++
		}
		start = e.Timestamp
		return true
	})
	if err != nil {
		return nil, 0, false, err
	}
	latest, err := s.em.Latest(tenantId, taskId)
	if err != nil {
		return nil, 0, false, nil
	}
	if latest.EventType == string(enum.StartTransition) {
		inTransition = true
	}
	last := &types.TaskRun{
		Result: latest.EventType, Status: enum.TaskRunStatusFailed,
		Error: latest.EventType, Start: start, End: latest.Timestamp,
	}
	return last, retried, inTransition, nil
}

// dispatch dispatches user tasks
func (s *Scheduler) dispatch(tasks entity.UserTasks) {
	ctx := context.Background()

	active := make([]string, 0)
	if len(tasks) > 0 {
		s.lg.Log(types.LevelInfo, "schedulerId", s.SchedulerId(), "taskIds", tasks.TaskIds(),
			"message", "dispatching tasks")
	}
	for _, task := range tasks {
		// 1. Check task status to avoid repeat dispatching Success/TaskRunStatusFailed/Canceled tasks
		status, _ := s.db.GetTaskStatus(ctx, types.GetTaskStatusOption{
			TaskType: enum.TaskTypeUserTask,
			Uid:      task.Uid,
		})
		if status == enum.TaskStatusSuccess || status == enum.TaskStatusFailed || status == enum.TaskStatusCanceled {
			s.lg.Log(types.LevelInfo, "tenantId", task.TenantId, "taskId", task.Uid, "status", status,
				"message", "the task has succeeded/failed/been canceled, can not be dispatched")
			continue
		}
		// 2. Dispatch the task

		// 2.1 Construct the task message, including task info, config, history
		v := types.Task{
			Handler:     task.Handler,
			TenantId:    task.TenantId,
			Uid:         task.Uid,
			SchedulerId: s.SchedulerId(),
			Type:        enum.TaskTypeUserTask,
		}
		active = append(active, task.TenantId)

		cfg, err := json.Marshal(task.Config)
		if err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", task.TenantId, "taskId", task.Uid, "message", "failed to marshal task config")
			continue
		}
		v.Config = cfg
		v.LastRun, v.RestartTimes, v.InTransition, err = s.taskHistory(task.TenantId, task.Uid)
		if err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", task.TenantId, "taskId", task.Uid, "message", "failed to lookup task history")
			continue
		}

		byt, err := json.Marshal(v)
		if err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", task.TenantId, "taskId", task.Uid, "message", "failed to marshal message")
			continue
		}

		// 2.2 Send task message through transport
		err = s.tran.Send(s.SchedulerId(), "", byt)
		if err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", task.TenantId, "taskId", task.Uid, "message", "failed to dispatch task")
			continue
		}

		// 3. Record the dispatch event and update task status accordingly
		ev := NewEventFromUserTask(TaskDispatched, task)
		if err = s.em.Insert(ev); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", task.TenantId, "taskId", task.Uid, "message", "failed to record task dispatch event")
		}
		if err = s.updateTaskStatus(ev); err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", task.TenantId, "taskId", task.Uid, "message", "failed to update task status")
		}
		le := types.ListenerEvent{SchedulerId: s.SchedulerId(), Task: types.NewTaskMetadataFromUserTaskEntity(task)}
		s.ls.OnTaskDispatching(le)
	}
	// 4. Update tenants' active state after dispatching
	if len(active) > 0 {
		err := s.db.ActivateTenants(context.Background(), types.ActivateTenantsOption{TenantId: active, ActiveTime: time.Now()})
		if err != nil {
			s.lg.Log(types.LevelError, "error", err.Error(), "message", "failed to activate tenants")
		}
		s.lg.Log(types.LevelDebug, "tenantIds", active, "message", "activated tenants")
	}
}

func (s *Scheduler) updateActiveTenants() (entity.Tenants, error) {
	start := time.Now().AddDate(0, 0, -7)
	scid := s.SchedulerId()
	opt := types.FindActiveTenantsOption{
		From:      &start,
		Zone:      &s.opt.Zone,
		Partition: &scid,
	}
	tenants, err := s.db.FindActiveTenants(context.Background(), opt)
	if err != nil {
		return nil, fmt.Errorf("failed to find active tenants: %w", err)
	}

	if len(tenants) == 0 {
		return tenants, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range tenants {
		if _, ok := s.cs[v.Uid]; !ok {
			s.lg.Log(types.LevelInfo, "tenantId", v.Uid, "message", "found new active tenant")
			s.cs[v.Uid] = queue.NewTaskQueue(s.db, s.lg, tenants[i], s.ls)
		} else {
			// Update tenant
			s.cs[v.Uid].Tenant = tenants[i]
		}
		s.em.CreateTenantBucket(v.Uid)
	}
	return tenants, nil
}

func (s *Scheduler) updateTaskStatus(ev *TaskEvent) error {
	var status enum.TaskStatus
	switch ev.EventType {
	case TaskDispatched:
		status = enum.TaskStatusDispatched
	case string(enum.TaskStarted), string(enum.FinishTransition):
		status = enum.TaskStatusRunning
	case string(enum.RetryTask):
		status = enum.TaskStatusNeedsRetry
	case string(enum.TaskFinished):
		status = enum.TaskStatusSuccess
	case string(enum.TaskFailed):
		status = enum.TaskStatusFailed
	case string(enum.StartTransition):
		status = enum.TaskStatusInTransition
	default:
		return nil
	}

	err := s.db.UpdateTaskStatus(context.Background(), types.UpdateTaskStatusOption{
		TaskType: ev.TaskType,
		Uids:     []string{ev.TaskId},
		Status:   status,
	})
	return err
}

func (s *Scheduler) checkStaleTasks() {
	var (
		ev    *TaskEvent
		tasks []string
		err   error
	)
	time.Sleep(time.Duration(s.opt.StaleCheckDelay) * time.Second)

	tick := time.NewTicker(time.Duration(s.opt.ScheduleInterval) * time.Second)
	for {
		select {
		case <-tick.C:
			for tenant, _ := range s.cs {
				tasks, err = s.em.Tasks(tenant)
				if err != nil {
					s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", tenant, "message", "error finding tasks")
					continue
				}

				for _, task := range tasks {
					ev, err = s.em.Latest(tenant, task)
					if err != nil {
						s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", tenant, "message", "error finding the latest event")
						continue
					}
					if !s.IsTaskTimeout(ev) {
						continue
					}

					s.lg.Log(types.LevelInfo, "tenantId", tenant, "taskId", task,
						"cause", fmt.Sprintf("task event %s has not been updated over %ds", ev.EventType, s.opt.TaskEventUpdateDeadline),
						"message", "found stale task")

					err = s.em.Delete(ev.TenantId, ev.TaskId)
					if err != nil {
						s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", tenant, "taskId", task,
							"message", "error deleting stale task event")
						continue
					}

					// check database task status if not finish scheduler again
					var taskStatus enum.TaskStatus
					taskStatus, err = s.db.GetTaskStatus(context.Background(), types.GetTaskStatusOption{TaskType: ev.TaskType, Uid: ev.TaskId})
					if err != nil {
						s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", tenant, "taskId", task,
							"message", "failed to get the newest status of the stale task")
						continue
					}
					if taskStatus != enum.TaskStatusSuccess && taskStatus != enum.TaskStatusFailed {
						if err = s.db.UpdateTaskStatus(context.Background(), types.UpdateTaskStatusOption{
							TaskType: ev.TaskType,
							Uids:     []string{ev.TaskId},
							Status:   enum.TaskStatusPending,
						}); err != nil {
							s.lg.Log(types.LevelError, "error", err.Error(), "tenantId", tenant, "taskId", task,
								"message", "failed to update task status")
							continue
						}
					}
				}
			}
		}
	}
}

func (s *Scheduler) IsTaskTimeout(ev *TaskEvent) bool {
	now := time.Now()
	if now.Before(ev.Timestamp) /* The event timestamp is in the future */ {
		return false
	}
	sec := int64(now.Sub(ev.Timestamp).Seconds())

	switch ev.EventType {
	case TaskDispatched:
		return sec > s.opt.TaskEventUpdateDeadline
	case string(enum.TaskStarted):
		return sec > s.opt.TaskEventUpdateDeadline
	case string(enum.TaskStatusRunning):
		return sec > s.opt.TaskEventUpdateDeadline
	case string(enum.ReportTaskStatus):
		return sec > s.opt.TaskEventUpdateDeadline
	case string(enum.RetryTask):
		return sec > s.opt.TaskEventUpdateDeadline
	case string(enum.FinishTransition):
		return sec > s.opt.TaskEventUpdateDeadline
	case string(enum.TaskFinished):
		return true
	default:
		return false
	}
}

func (s *Scheduler) SchedulerId() string {
	return strings.ToLower(fmt.Sprintf("%v-%v", s.opt.Zone, s.opt.Name))
}

// TODO: ?
func (s *Scheduler) resetSchedulingTask() {
	ids := make([]string, 0)
	for _, q := range s.cs {
		tasks, err := q.PopAllUserTasks()
		if err != nil {
			continue
		}
		for _, task := range tasks {
			ids = append(ids, task.Uid)
		}
	}
	err := s.db.UpdateTaskStatus(context.Background(), types.UpdateTaskStatusOption{
		TaskType: enum.TaskTypeUserTask,
		Uids:     ids,
		Status:   enum.TaskStatusPending,
	})
	if err != nil {
		s.lg.Log(types.LevelError, "error", err.Error(), "taskIds", ids,
			"message", "failed to reset task status to Pending before shutting down")
	} else {
		s.lg.Log(types.LevelInfo, "taskIds", ids,
			"message", "reset task status to Pending before shutting down")
	}
}

// printStack logs exception stack
func (s *Scheduler) printStack(err interface{}) {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	s.lg.Log(types.LevelError, "schedulerId", s.SchedulerId(), "error", err,
		"stack", string(buf[:n]), "message", "scheduler panicked")
}
