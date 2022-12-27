package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	listener2 "github.com/mykube-run/keel/pkg/impl/listener"
	"github.com/mykube-run/keel/pkg/impl/transport"
	"github.com/mykube-run/keel/pkg/logger"
	"github.com/mykube-run/keel/pkg/queue"
	"github.com/mykube-run/keel/pkg/types"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Options struct {
	Name             string                 // Scheduler name, also used as partition name
	Zone             string                 // Zone name
	ScheduleInterval int64                  // Interval in seconds for checking active tenants & new tasks
	StaleCheckDelay  int64                  // Time in seconds for checking stale tasks
	TaskTimeoutTime  int64                  // Specifies the timeout period for the scheduler to receive messages Unit：seconds
	Snapshot         config.SnapshotConfig  // Scheduler state snapshot configurations
	Transport        config.TransportConfig // Transport config
	ServerConfig     config.ServerConfig    // http and grpc config
}

type Scheduler struct {
	opt       *Options
	mu        sync.Mutex
	cs        map[string]*queue.TaskQueue
	em        *EventManager
	db        types.DB
	lg        logger.Logger
	srv       *Server
	tran      types.Transport
	listener  types.Listener
	closeChan chan struct{}
}

func New(opt *Options, db types.DB, lg logger.Logger, listener types.Listener) (s *Scheduler, err error) {
	if listener == nil {
		listener = listener2.DefaultListener{}
	}
	s = &Scheduler{
		opt:       opt,
		cs:        make(map[string]*queue.TaskQueue),
		db:        db,
		lg:        lg,
		listener:  listener,
		closeChan: make(chan struct{}),
	}
	s.srv = NewServer(db, s, opt.ServerConfig, lg, listener)
	s.em, err = NewEventManager(opt.Snapshot, s.SchedulerId(), lg)
	if err != nil {
		return nil, err
	}

	opt.Transport.Kafka.GroupId = s.SchedulerId() /* Update group id */
	s.tran, err = transport.New(&opt.Transport)
	if err != nil {
		return nil, err
	}
	s.tran.OnReceive(s.onReceiveMessage)
	return s, nil
}

func (s *Scheduler) Start() {
	_ = s.lg.Log(logger.LevelInfo, "SchedulerId", s.SchedulerId(), "transport", s.opt.Transport.Type, "message", "starting scheduler")
	_, _ = s.updateActiveTenants()
	go s.schedule()
	go s.checkStaleTasks()
	go s.srv.Start()

	stopC := make(chan os.Signal)
	signal.Notify(stopC, os.Interrupt, syscall.SIGTERM /* SIGTERM is expected inside k8s */)

	select {
	case <-stopC:
		_ = s.lg.Log(logger.LevelInfo, "message", "received stop signal")
		//close schedule
		s.closeChan <- struct{}{}
		s.RecoverSchedulingTask()
		key, err := s.em.Backup()
		_ = s.lg.Log(logger.LevelInfo, "key", key, "err", err.Error(), "message", "saved events db snapshot")
	}
}

func (s *Scheduler) schedule() {
	tick := time.NewTicker(time.Duration(s.opt.ScheduleInterval) * time.Second)
	for {
		select {
		case <-s.closeChan:
			return
		case <-tick.C:
			if _, err := s.updateActiveTenants(); err != nil {
				_ = s.lg.Log(logger.LevelError, "message", "failed to update active tenants")
			}
			for k, c := range s.cs {
				running, err := s.em.CountRunningTasks(k)
				if err != nil {
					_ = s.lg.Log(logger.LevelError, "message", "failed to count running tasks")
					continue
				}
				if !c.Tenant.ResourceQuota.Concurrency.Valid {
					_ = s.lg.Log(logger.LevelWarn, "message", "tenant resource quota concurrency was invalid")
					continue
				}
				_ = s.lg.Log(logger.LevelDebug, "tenantId", k, "quota", c.Tenant.ResourceQuota.Concurrency.Int64,
					"running", running, "message", "tenant concurrency state")
				n := int(c.Tenant.ResourceQuota.Concurrency.Int64) - running
				if n <= 0 {
					continue
				}
				tasks, _, err := c.PopUserTasks(n)
				if err != nil {
					_ = s.lg.Log(logger.LevelError, "message", "failed to pop user tasks from local task queue")
					continue
				}
				s.dispatch(tasks)
			}
		}
	}
}

func (s *Scheduler) onReceiveMessage(from string, msg []byte) ([]byte, error) {
	var m types.TaskMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		_ = s.lg.Log(logger.LevelError, "message", "failed to unmarshal task message")
		return nil, err
	}

	s.handleTaskMessage(&m)
	return nil, nil
}

func (s *Scheduler) handleTaskMessage(m *types.TaskMessage) {
	if m == nil {
		return
	}

	ev := NewEventFromMessage(m)
	if err := s.em.Insert(ev); err != nil {
		_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "message", "failed to insert task event")
	}
	_ = s.lg.Log(logger.LevelInfo, "tenantId", ev.TenantId, "taskId", ev.TaskId, "event", ev.EventType, "message", "get Task Message")
	switch m.Type {
	case enum.RetryTask:
		msg := types.ListenerEventMessage{TenantUID: ev.TenantId, TaskUID: ev.TaskId}
		s.listener.OnTaskNeedRetry(msg)
		_ = s.lg.Log(logger.LevelWarn, "tenantId", ev.TenantId, "taskId", ev.TaskId, "message", "task needs retry")
		_, ok := s.cs[ev.TenantId]
		if !ok {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "message", "received message from illegal tenant")
			return
		}
		s.dispatch([]*entity.UserTask{{TenantId: m.Task.TenantId, Uid: m.Task.Uid,
			Handler: m.Task.Handler, Config: m.Task.Config},
		})
	case enum.TaskFailed:
		if err := s.updateTaskStatus(ev); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "message", "failed to update tasks status")
		}
		if err := s.em.Delete(ev.TenantId, ev.TaskId); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "err", err.Error(),
				"message", "failed to delete task events")
		}
		_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId",
			ev.TaskId, "message", "task run fail")
	case enum.ReportTaskStatus:
		// Do Nothing
		return
	case enum.StartTransition:
		// mark task transition and wait worker to FinishTransition
		if err := s.updateTaskStatus(ev); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "message",
				"failed to update tasks status")
		}
		s.dispatch([]*entity.UserTask{{
			TenantId: m.Task.TenantId, Uid: m.Task.Uid,
			Handler: m.Task.Handler, Config: m.Task.Config},
		})
	case enum.FinishTransition:
		// mark task Running
		if err := s.updateTaskStatus(ev); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "message",
				"failed to update tasks status")
		}
	case enum.TaskStarted:
		if err := s.updateTaskStatus(ev); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "message",
				"failed to update tasks status")
		}
		msg := types.ListenerEventMessage{TenantUID: ev.TenantId, TaskUID: ev.TaskId}
		s.listener.OnTaskRunning(msg)
		_ = s.lg.Log(logger.LevelInfo, "tenantId", ev.TenantId, "taskId",
			ev.TaskId, "message", "worker starting to process task")
	case enum.TaskFinished:
		if err := s.updateTaskStatus(ev); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "message",
				"failed to update tasks status")
		}
		msg := types.ListenerEventMessage{TenantUID: ev.TenantId, TaskUID: ev.TaskId}
		s.listener.OnTaskFinished(msg)
		_ = s.lg.Log(logger.LevelInfo, "tenantId", ev.TenantId, "taskId",
			ev.TaskId, "message", "worker has finished processing task")
		if err := s.em.Delete(ev.TenantId, ev.TaskId); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "err", err.Error(),
				"message", "failed to delete task events")
		}
		runningCount, _ := s.em.CountRunningTasks(ev.TenantId)
		_ = s.lg.Log(logger.LevelDebug, "tenantId", ev.TenantId, "taskId", "runningTasks", runningCount,
			"message", "failed to delete task events")
	}
}

func (s *Scheduler) taskHistory(tenantId, taskId string) (*types.TaskRun, int, bool, error) {
	var (
		retried        = 0
		start          time.Time
		needTransition = false
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
		needTransition = true
	}
	last := &types.TaskRun{
		Result: latest.EventType, Status: enum.TaskRunStatusFailed,
		Error: latest.EventType, Start: start, End: latest.Timestamp,
	}
	return last, retried, needTransition, nil
}

// dispatch dispatches user tasks
func (s *Scheduler) dispatch(tasks entity.UserTasks) {
	ctx := context.Background()

	activeTenants := make([]string, 0)
	if len(tasks) > 0 {
		// Log event
		_ = s.lg.Log(logger.LevelInfo, "SchedulerId", s.SchedulerId(), "taskId", tasks.TaskIds(),
			"message", "dispatch tasks")
	}
	for _, task := range tasks {
		// 1. Check task status to avoid repeat dispatching Success/TaskRunStatusFailed/Canceled tasks
		status, _ := s.db.GetTaskStatus(ctx, types.GetTaskStatusOption{
			TaskType: enum.TaskTypeUserTask,
			Uid:      task.Uid,
		})
		if status == enum.TaskStatusSuccess || status == enum.TaskStatusFailed || status == enum.TaskStatusCanceled {
			_ = s.lg.Log(logger.LevelInfo, "status", status, "message", "the task can not be dispatched")
			continue
		}
		// 2. Dispatch the task
		v := types.Task{
			Handler:     task.Handler,
			TenantId:    task.TenantId,
			Uid:         task.Uid,
			SchedulerId: s.SchedulerId(),
			Type:        enum.TaskTypeUserTask,
		}
		// if dispatch this task need active tenant
		activeTenants = append(activeTenants, task.TenantId)

		cfg, err := json.Marshal(task.Config)
		if err != nil {
			_ = s.lg.Log(logger.LevelError, "message", "failed to marshal task config")
			continue
		}
		v.Config = cfg
		v.LastRun, v.RestartTimes, v.NeedRunWithTransition, err = s.taskHistory(task.TenantId, task.Uid)
		if err != nil {
			_ = s.lg.Log(logger.LevelError, "message", "failed to lookup task history")
			continue
		}

		byt, err := json.Marshal(v)
		if err != nil {
			_ = s.lg.Log(logger.LevelError, "message", "failed to marshal message")
			continue
		}

		err = s.tran.Send(s.SchedulerId(), "", byt)
		if err != nil {
			_ = s.lg.Log(logger.LevelError, "message", "failed to dispatch task")
			continue
		}

		// 3. Record the dispatch event and update task status accordingly
		ev := NewEventFromUserTask(TaskDispatched, task)
		if err = s.em.Insert(ev); err != nil {
			_ = s.lg.Log(logger.LevelInfo, "err", err.Error(), "message", "failed to record task dispatch event")
		}
		if err = s.updateTaskStatus(ev); err != nil {
			_ = s.lg.Log(logger.LevelInfo, "err", err.Error(), "message", "failed to update task status")
		}
		msg := types.ListenerEventMessage{TenantUID: task.TenantId, TaskUID: task.Uid}
		s.listener.OnTaskDispatching(msg)
	}
	//when dispatch task for a loop active tenants
	if len(activeTenants) > 0 {
		err := s.db.ActiveTenant(context.Background(), types.ActiveTenantOption{TenantId: activeTenants, ActiveTime: time.Now()})
		if err != nil {
			_ = s.lg.Log(logger.LevelInfo, "err", err.Error(), "message", "active tenant fail")
		}
		_ = s.lg.Log(logger.LevelDebug, "tenantIds", activeTenants, "message", "active tenants")
	}
}

func (s *Scheduler) updateActiveTenants() (entity.Tenants, error) {
	start := time.Now().AddDate(0, 0, -7)
	opt := types.FindActiveTenantsOption{
		From: &start,
		Zone: &s.opt.Zone,
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
			_ = s.lg.Log(logger.LevelInfo, "tenantId", v.Uid, "message", "found new active tenant")
			s.cs[v.Uid] = queue.NewTaskQueue(s.db, s.lg, tenants[i], s.listener)
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
					_ = s.lg.Log(logger.LevelError, "err", err.Error(), "message", "error finding tasks")
					continue
				}

				for _, task := range tasks {
					ev, err = s.em.Latest(tenant, task)
					if err != nil {
						_ = s.lg.Log(logger.LevelError, "err", err.Error(), "message", "error finding the latest event")
						continue
					}
					ok := s.IsTaskTimeout(ev)
					if !ok {
						continue
					}
					_ = s.lg.Log(logger.LevelInfo, "taskId", task, "cause", fmt.Sprintf("task %s over %d second no signal", ev.EventType, s.opt.TaskTimeoutTime), "message", "find StaleTasks")
					err = s.em.Delete(ev.TenantId, ev.TaskId)
					if err != nil {
						_ = s.lg.Log(logger.LevelDebug, "err", err.Error(), "message", "delete tenant error")
						continue
					}
					// check database task status if not finish scheduler again
					var taskStatus enum.TaskStatus
					taskStatus, err = s.db.GetTaskStatus(context.Background(), types.GetTaskStatusOption{TaskType: ev.TaskType, Uid: ev.TaskId})
					if err != nil {
						_ = s.lg.Log(logger.LevelDebug, "err", err.Error(), "message", "get deathTask state error")
						continue
					}
					if taskStatus != enum.TaskStatusSuccess && taskStatus != enum.TaskStatusFailed {
						if err = s.db.UpdateTaskStatus(context.Background(), types.UpdateTaskStatusOption{
							TaskType: ev.TaskType,
							Uids:     []string{ev.TaskId},
							Status:   enum.TaskStatusPending,
						}); err != nil {
							_ = s.lg.Log(logger.LevelDebug, "err", err.Error(), "message", "failed to update task status")
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
		return sec > s.opt.TaskTimeoutTime
	case string(enum.TaskStarted):
		return sec > s.opt.TaskTimeoutTime
	case string(enum.TaskStatusRunning):
		return sec > s.opt.TaskTimeoutTime
	case string(enum.ReportTaskStatus):
		return sec > s.opt.TaskTimeoutTime
	case string(enum.RetryTask):
		return sec > s.opt.TaskTimeoutTime
	case string(enum.FinishTransition):
		return sec > s.opt.TaskTimeoutTime
	case string(enum.TaskFinished):
		return true
	default:
		return false
	}
}

func (s *Scheduler) SchedulerId() string {
	return strings.ToLower(fmt.Sprintf("%v-%v", s.opt.Zone, s.opt.Name))
}

func (s *Scheduler) RecoverSchedulingTask() {
	allSchedulingTask := make([]string, 0)
	for _, taskQueue := range s.cs {
		tasks, err := taskQueue.PopAllUserTasks()
		if err != nil {
			continue
		}
		for _, task := range tasks {
			allSchedulingTask = append(allSchedulingTask, task.Uid)
		}
	}
	err := s.db.UpdateTaskStatus(context.Background(), types.UpdateTaskStatusOption{
		TaskType: enum.TaskTypeUserTask,
		Uids:     allSchedulingTask,
		Status:   enum.TaskStatusPending,
	})
	if err != nil {
		_ = s.lg.Log(logger.LevelError, "err", err.Error(), "message", "pod close ture Scheduling Task to Pending success")
		return
	}
	_ = s.lg.Log(logger.LevelInfo, "success", len(allSchedulingTask), "message", "")
}
