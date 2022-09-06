package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/transport"
	"github.com/mykube-run/keel/pkg/queue"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog"
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
	opt  *Options
	mu   sync.Mutex
	cs   map[string]*queue.TaskQueue
	em   *EventManager
	db   types.DB
	lg   *zerolog.Logger
	srv  *server
	tran types.Transport
}

func New(opt *Options, db types.DB, lg *zerolog.Logger) (s *Scheduler, err error) {
	s = &Scheduler{
		opt: opt,
		cs:  make(map[string]*queue.TaskQueue),
		db:  db,
		lg:  lg,
	}
	s.srv = NewServer(db, s, opt.ServerConfig)
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
	s.lg.Info().Str("SchedulerId", s.SchedulerId()).Str("transport", s.opt.Transport.Type).Msg("starting scheduler")

	_, _ = s.updateActiveTenants()

	go s.schedule()
	go s.checkStaleTasks()
	go s.srv.Start()

	stopC := make(chan os.Signal)
	signal.Notify(stopC, os.Interrupt, syscall.SIGTERM /* SIGTERM is expected inside k8s */)

	select {
	case <-stopC:
		s.lg.Info().Msg("received stop signal")
		key, err := s.em.Backup()
		s.lg.Info().Str("key", key).Err(err).Msg("saved events db snapshot")
		_ = s.tran.Close()
	}
}

func (s *Scheduler) schedule() {
	tick := time.NewTicker(time.Duration(s.opt.ScheduleInterval) * time.Second)
	for {
		select {
		case <-tick.C:
			if _, err := s.updateActiveTenants(); err != nil {
				s.lg.Err(err).Msg("failed to update active tenants")
			}

			for k, c := range s.cs {
				running, err := s.em.CountTasks(k)
				if err != nil {
					s.lg.Err(err).Msg("failed to count running tasks")
					continue
				}
				if !c.Tenant.ResourceQuota.Concurrency.Valid {
					s.lg.Warn().Msg("tenant resource quota concurrency was invalid")
					continue
				}
				s.lg.Trace().Str("tenantId", k).Int64("quota", c.Tenant.ResourceQuota.Concurrency.Int64).
					Int("running", running).Msg("tenant concurrency state")

				n := int(c.Tenant.ResourceQuota.Concurrency.Int64) - running
				if n <= 0 {
					continue
				}

				tasks, _, err := c.PopUserTasks(n)
				if err != nil {
					s.lg.Err(err).Msg("failed to pop user tasks from local task queue")
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
		s.lg.Err(err).Msg("failed to unmarshal task message")
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
		s.lg.Err(err).Str("tenantId", ev.TenantId).Str("taskId", ev.TaskId).Msg("failed to insert task event")
	}
	if err := s.updateTaskStatus(ev); err != nil {
		s.lg.Err(err).Str("tenantId", ev.TenantId).Str("taskId", ev.TaskId).Msg("failed to update tasks status")
	}

	switch m.Type {
	case enum.RetryTask:
		s.lg.Warn().Str("tenantId", ev.TenantId).Str("taskId", ev.TaskId).Msg("task needs retry")
		//tasks, err := s.db.GetTask(context.Background(), types.GetTaskOption{
		//	TaskType: m.Task.Type,
		//	Uid:      m.Task.Uid,
		//})
		//if err != nil {
		//	s.lg.Err(err).Str("tenantId", ev.TenantId).Str("taskId", ev.TaskId).Msg("error getting task from database")
		//	return
		//}
		//
		//tc, ok := s.cs[ev.TenantId]
		//if !ok {
		//	s.lg.Warn().Str("tenantId", ev.TenantId).Str("taskId", ev.TaskId).Msg("tenant task cache does not exist, can not be retried")
		//	return
		//}
		//switch m.Task.Type {
		//case entity.TaskTypeUserTask:
		//	if len(tasks.UserTasks) != 1 {
		//		s.lg.Warn().Str("tenantId", ev.TenantId).Str("taskId", ev.TaskId).Msg("found no task")
		//		return
		//	}
		//	if err = s.db.UpdateTaskStatus(context.Background(), types.UpdateTaskStatusOption{
		//		TaskType: entity.TaskTypeUserTask,
		//		Uids:     tasks.UserTasks.TaskIds(),
		//		Status:   entity.TaskStatusScheduling,
		//	}); err != nil {
		//		return
		//	}
		//	t := tasks.UserTasks[0]
		//	tc.EnqueueUserTask(t, 1)
		//}
	case enum.ReportTaskStatus:
		// Does nothing
	case enum.TaskStarted:
		s.lg.Info().Str("tenantId", ev.TenantId).Str("taskId", ev.TaskId).Msg("worker starting to process task")
	case enum.TaskFinished:
		s.lg.Info().Str("tenantId", ev.TenantId).Str("taskId", ev.TaskId).Msg("worker has finished processing task")
		if err := s.em.Delete(ev.TenantId, ev.TaskId); err != nil {
			s.lg.Err(err).Msg("failed to delete task events")
		}
		// Dispatch one task
		c, ok := s.cs[ev.TenantId]
		if !ok {
			return
		}
		tasks, _, err := c.PopUserTasks(1)
		if err != nil {
			s.lg.Err(err).Msg("failed to pop user tasks from local cache")
			return
		}
		s.dispatch(tasks)
	}
}

func (s *Scheduler) taskHistory(tenantId, taskId string) (*types.TaskRun, int, error) {
	n := -1
	last := new(types.TaskRun)

	fn := func(e *TaskEvent) bool {
		switch e.EventType {
		case string(enum.TaskStarted):
			n += 1
			last.Start = e.Timestamp
		case string(enum.ReportTaskStatus):
			last.Status = enum.TaskRunStatusRunning
			// TODO: update progress
		case string(enum.TaskFinished):
			last.Status = enum.TaskRunStatusSucceed
			last.End = e.Timestamp
			// TODO: update result & error
		case string(enum.TaskFailed):
			last.Status = enum.TaskRunStatusFailed
			last.End = e.Timestamp
			// TODO: update result & error
		}
		return true
	}

	err := s.em.Iterate(tenantId, taskId, fn)
	return last, n, err
}

// dispatch dispatches user tasks
func (s *Scheduler) dispatch(tasks entity.UserTasks) {
	ctx := context.Background()

	for _, task := range tasks {
		// Log event
		le := s.lg.Info().Str("SchedulerId", s.SchedulerId()).Str("tenantId", task.TenantId).
			Str("taskId", task.Uid)

		// 1. Check task status to avoid repeat dispatching Success/TaskRunStatusFailed/Canceled tasks
		status, _ := s.db.GetTaskStatus(ctx, types.GetTaskStatusOption{
			TaskType: enum.TaskTypeUserTask,
			Uid:      task.Uid,
		})
		if status == enum.TaskStatusSuccess || status == enum.TaskStatusFailed || status == enum.TaskStatusCanceled {
			le.Interface("status", status).Msg("the task can not be dispatched")
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

		cfg, err := json.Marshal(task.Config)
		if err != nil {
			le.Err(err).Msg("failed to marshal task config")
			continue
		}
		v.Config = cfg
		v.LastRun, v.RestartTimes, err = s.taskHistory(task.TenantId, task.Uid)
		if err != nil {
			le.Err(err).Msg("failed to lookup task history")
			continue
		}

		byt, err := json.Marshal(v)
		if err != nil {
			le.Err(err).Msg("failed to marshal message")
			continue
		}

		err = s.tran.Send(s.SchedulerId(), "Any", byt)
		if err != nil {
			le.Err(err).Msg("failed to dispatch task")
			continue
		}

		// 3. Record the dispatch event and update task status accordingly
		ev := NewEventFromUserTask(TaskDispatched, task)
		le.Msg("dispatched task to workers")

		if err = s.em.Insert(ev); err != nil {
			le.Err(err).Msg("failed to record task dispatch event")
		}
		if err = s.updateTaskStatus(ev); err != nil {
			le.Err(err).Msg("failed to update task status")
		}
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
			s.lg.Info().Str("tenantId", v.Uid).Msg("found new active tenant")
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
					s.lg.Err(err).Msg("error finding tasks")
					continue
				}

				for _, task := range tasks {
					ev, err = s.em.Latest(tenant, task)
					if err != nil {
						s.lg.Err(err).Msg("error finding the latest event")
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
						s.lg.Err(err).Msg("failed to update task status")
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
