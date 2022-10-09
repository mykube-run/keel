package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
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
	Snapshot         config.SnapshotConfig  // Scheduler state snapshot configurations
	Transport        config.TransportConfig // Transport config
	ServerConfig     config.ServerConfig    // http and grpc config
}

type Scheduler struct {
	opt              *Options
	mu               sync.Mutex
	cs               map[string]*queue.TaskQueue
	em               *EventManager
	db               types.DB
	lg               logger.Logger
	srv              *server
	tran             types.Transport
	workerActiveTime map[string]time.Time //key is worker id, value is lastActiveTime
	aliveWorker      []string             // all alive workers id
}

func New(opt *Options, db types.DB, lg logger.Logger) (s *Scheduler, err error) {
	s = &Scheduler{
		opt: opt,
		cs:  make(map[string]*queue.TaskQueue),
		db:  db,
		lg:  lg,
	}
	s.srv = NewServer(db, s, opt.ServerConfig, lg)
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
	s.workerActiveTime = make(map[string]time.Time)
	s.aliveWorker = make([]string, 0)
	return s, nil
}

func (s *Scheduler) Start() {
	_ = s.lg.Log(logger.LevelInfo, "SchedulerId", s.SchedulerId(), "transport", s.opt.Transport.Type, "msg", "starting scheduler")
	_, _ = s.updateActiveTenants()
	go s.schedule()
	go s.checkStaleTasks()
	go s.srv.Start()

	stopC := make(chan os.Signal)
	signal.Notify(stopC, os.Interrupt, syscall.SIGTERM /* SIGTERM is expected inside k8s */)

	select {
	case <-stopC:
		_ = s.lg.Log(logger.LevelInfo, "msg", "received stop signal")
		key, err := s.em.Backup()
		_ = s.lg.Log(logger.LevelInfo, "key", key, "err", err.Error(), "msg", "saved events db snapshot")
		_ = s.tran.Close()
	}
}

func (s *Scheduler) schedule() {
	tick := time.NewTicker(time.Duration(s.opt.ScheduleInterval) * time.Second)
	for {
		select {
		case <-tick.C:
			if _, err := s.updateActiveTenants(); err != nil {
				_ = s.lg.Log(logger.LevelError, "msg", "failed to update active tenants")
			}
			for k, c := range s.cs {
				running, err := s.em.CountTasks(k)
				if err != nil {
					_ = s.lg.Log(logger.LevelError, "msg", "failed to count running tasks")
					continue
				}
				if !c.Tenant.ResourceQuota.Concurrency.Valid {
					_ = s.lg.Log(logger.LevelWarn, "msg", "tenant resource quota concurrency was invalid")
					continue
				}
				_ = s.lg.Log(logger.LevelDebug, "tenantId", k, "quota", c.Tenant.ResourceQuota.Concurrency.Int64,
					"running", running, "msg", "tenant concurrency state")
				n := int(c.Tenant.ResourceQuota.Concurrency.Int64) - running
				if n <= 0 {
					continue
				}
				tasks, _, err := c.PopUserTasks(n)
				if err != nil {
					_ = s.lg.Log(logger.LevelError, "msg", "failed to pop user tasks from local task queue")
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
		_ = s.lg.Log(logger.LevelError, "msg", "failed to unmarshal task message")
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
		_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "msg", "failed to insert task event")
	}
	if err := s.updateTaskStatus(ev); err != nil {
		_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "msg", "failed to update tasks status")
	}
	s.workerActiveTime[m.WorkerId] = time.Now()
	switch m.Type {
	case enum.RetryTask:
		_ = s.lg.Log(logger.LevelWarn, "tenantId", ev.TenantId, "taskId", ev.TaskId, "msg", "task needs retry")
		_, ok := s.cs[ev.TenantId]
		if !ok {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "msg", "received message from illegal tenant")
			return
		}
		s.dispatch([]*entity.UserTask{{TenantId: m.Task.TenantId, Uid: m.Task.Uid,
			Handler: m.Task.Handler, Config: m.Task.Config},
		})
	case enum.TaskFailed:
		if err := s.em.Delete(ev.TenantId, ev.TaskId); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "err", err.Error(),
				"msg", "failed to delete task events")
		}
		_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId",
			ev.TaskId, "msg", "task run fail")
	case enum.ReportTaskStatus:
		// Does nothing
	case enum.TaskStarted:
		_ = s.lg.Log(logger.LevelInfo, "tenantId", ev.TenantId, "taskId",
			ev.TaskId, "msg", "worker starting to process task")
	case enum.TaskFinished:
		_ = s.lg.Log(logger.LevelInfo, "tenantId", ev.TenantId, "taskId",
			ev.TaskId, "msg", "worker has finished processing task")
		if err := s.em.Delete(ev.TenantId, ev.TaskId); err != nil {
			_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "err", err.Error(),
				"msg", "failed to delete task events")
		}
		runningCount, _ := s.em.CountTasks(ev.TenantId)
		_ = s.lg.Log(logger.LevelDebug, "tenantId", ev.TenantId, "taskId", "runningTasks", runningCount,
			"msg", "failed to delete task events")
		// Dispatch one task
		//c, ok := s.cs[ev.TenantId]
		//if !ok {
		//	return
		//}
		//tasks, _, err := c.PopUserTasks(1)
		//if err != nil {
		//	_ = s.lg.Log(logger.LevelError, "tenantId", ev.TenantId, "taskId", ev.TaskId, "err", err.Error(),
		//		"msg", "failed to pop user tasks from local cache")
		//	return
		//}
		//s.dispatch(tasks)
	}
}

func (s *Scheduler) taskHistory(tenantId, taskId string) (*types.TaskRun, int, error) {
	var (
		retried = 0
		start   time.Time
	)

	err := s.em.Iterate(tenantId, taskId, func(e *TaskEvent) bool {
		switch e.EventType {
		case string(enum.RetryTask):
			retried++
		case string(enum.TaskStarted):
			start = e.Timestamp
		}
		return true
	})
	if err != nil {
		return nil, 0, err
	}
	latest, err := s.em.Latest(tenantId, taskId)
	if err != nil {
		return nil, 0, nil
	}
	last := &types.TaskRun{Result: "", Status: enum.TaskRunStatusFailed, Error: "", Start: start, End: latest.Timestamp}
	return last, retried, nil
}

// dispatch dispatches user tasks
func (s *Scheduler) dispatch(tasks entity.UserTasks) {
	ctx := context.Background()

	activeTenants := make([]string, 0)
	if len(tasks) > 0 {
		// Log event
		_ = s.lg.Log(logger.LevelInfo, "SchedulerId", s.SchedulerId(), "taskId", tasks.TaskIds(),
			"msg", "dispatch tasks")
	}
	for _, task := range tasks {
		// 1. Check task status to avoid repeat dispatching Success/TaskRunStatusFailed/Canceled tasks
		status, _ := s.db.GetTaskStatus(ctx, types.GetTaskStatusOption{
			TaskType: enum.TaskTypeUserTask,
			Uid:      task.Uid,
		})
		if status == enum.TaskStatusSuccess || status == enum.TaskStatusFailed || status == enum.TaskStatusCanceled {
			_ = s.lg.Log(logger.LevelInfo, "status", status, "msg", "the task can not be dispatched")
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
			_ = s.lg.Log(logger.LevelError, "msg", "failed to marshal task config")
			continue
		}
		v.Config = cfg
		v.LastRun, v.RestartTimes, err = s.taskHistory(task.TenantId, task.Uid)
		if err != nil {
			_ = s.lg.Log(logger.LevelError, "msg", "failed to lookup task history")
			continue
		}

		byt, err := json.Marshal(v)
		if err != nil {
			_ = s.lg.Log(logger.LevelError, "msg", "failed to marshal message")
			continue
		}

		err = s.tran.Send(s.SchedulerId(), "Any", byt)
		if err != nil {
			_ = s.lg.Log(logger.LevelError, "msg", "failed to dispatch task")
			continue
		}

		// 3. Record the dispatch event and update task status accordingly
		ev := NewEventFromUserTask(TaskDispatched, task)
		if err = s.em.Insert(ev); err != nil {
			_ = s.lg.Log(logger.LevelInfo, "err", err.Error(), "msg", "failed to record task dispatch event")
		}
		if err = s.updateTaskStatus(ev); err != nil {
			_ = s.lg.Log(logger.LevelInfo, "err", err.Error(), "msg", "failed to update task status")
		}
	}
	//when dispatch task for a loop active tenants
	if len(activeTenants) > 0 {
		err := s.db.ActiveTenant(context.Background(), types.ActiveTenantOption{TenantId: activeTenants, ActiveTime: time.Now()})
		if err != nil {
			_ = s.lg.Log(logger.LevelInfo, "err", err.Error(), "msg", "active tenant fail")
		}
		_ = s.lg.Log(logger.LevelDebug, "tenantIds", activeTenants, "msg", "active tenants")
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
			_ = s.lg.Log(logger.LevelInfo, "tenantId", v.Uid, "msg", "found new active tenant")
			s.cs[v.Uid] = queue.NewTaskQueue(s.db, s.lg, tenants[i])
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
					_ = s.lg.Log(logger.LevelDebug, "err", err.Error(), "msg", "error finding tasks")
					continue
				}

				for _, task := range tasks {
					ev, err = s.em.Latest(tenant, task)
					if err != nil {
						_ = s.lg.Log(logger.LevelDebug, "err", err.Error(), "msg", "error finding the latest event")
						continue
					}
					status, ok := s.shouldRevive(ev)
					if !ok {
						continue
					}
					err = s.db.UpdateTaskStatus(context.Background(), types.UpdateTaskStatusOption{
						TaskType: ev.TaskType,
						Uids:     []string{ev.TaskId},
						Status:   status,
					})
					if err != nil {
						_ = s.lg.Log(logger.LevelDebug, "err", err.Error(), "msg", "failed to update task status")
						continue
					}
					_ = s.em.Delete(tenant, task)
				}
			}
		}
	}
}

// shouldRevive checks whether event is outdated, returns the expected next status
func (s *Scheduler) shouldRevive(ev *TaskEvent) (enum.TaskStatus, bool) {
	now := time.Now()
	if now.Before(ev.Timestamp) /* The event timestamp is in the future */ {
		return "", false
	}
	sec := int(now.Sub(ev.Timestamp).Seconds())

	switch ev.EventType {
	case TaskDispatched:
		return enum.TaskStatusPending, sec > 60
	case string(enum.TaskStarted):
		return enum.TaskStatusPending, sec > 60
	case string(enum.ReportTaskStatus):
		return enum.TaskStatusPending, sec > 60
	case string(enum.RetryTask):
		return enum.TaskStatusPending, sec > 120
	case string(enum.StartTransition):
		return enum.TaskStatusPending, sec > 60
	case string(enum.FinishTransition):
		return enum.TaskStatusPending, sec > 60
	case string(enum.TaskFinished):
		return "", false
	default:
		return "", false
	}
}

func (s *Scheduler) SchedulerId() string {
	return strings.ToLower(fmt.Sprintf("%v-%v", s.opt.Zone, s.opt.Name))
}
