package worker

import (
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/transport"
	"github.com/mykube-run/keel/pkg/logger"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// TODO: Trigger task transition on exit signal

type Options struct {
	PoolSize       int                    // Worker pool size
	Name           string                 // Worker name, normally pod name
	Generation     int64                  // Worker generation
	ReportInterval time.Duration          // Interval for worker to report tasks status to scheduler, default to 5 seconds
	Transport      config.TransportConfig // Worker transport config
}

func (o *Options) validate() {
	if o.ReportInterval.Seconds() <= 0 {
		o.ReportInterval = time.Second * 5
	}
}

type Worker struct {
	lg        logger.Logger
	opt       *Options
	pool      *ants.Pool
	tran      types.Transport
	info      *types.WorkerInfo
	running   sync.Map
	factories map[string]types.TaskHandlerFactory
}

// New initializes a new Worker
func New(opt *Options, lg logger.Logger) (*Worker, error) {
	opt.validate()

	var err error
	w := &Worker{
		lg:        lg,
		factories: make(map[string]types.TaskHandlerFactory),
		opt:       opt,
		info: &types.WorkerInfo{
			Id:         opt.Name,
			Generation: opt.Generation,
		},
	}

	w.pool, err = ants.NewPool(opt.PoolSize)
	if err != nil {
		return nil, err
	}

	w.tran, err = transport.New(&opt.Transport)
	if err != nil {
		return nil, err
	}

	w.tran.OnReceive(w.onReceiveMessage)
	return w, nil
}

// RegisterHandler registers a TaskHandlerFactory for specified handler name
func (w *Worker) RegisterHandler(name string, f types.TaskHandlerFactory) {
	w.factories[name] = f
}

// Start starts the worker
func (w *Worker) Start() {
	_ = w.lg.Log(logger.LevelInfo, "workerId", w.opt.Name, "poolSize", w.opt.PoolSize, "msg", "starting worker")
	go w.report()

	stopC := make(chan os.Signal)
	signal.Notify(stopC, os.Interrupt, syscall.SIGTERM /* SIGTERM is expected inside k8s */)

	select {
	case <-stopC:
		_ = w.lg.Log(logger.LevelInfo, "msg", "received stop signal")
		_ = w.tran.Close()
		w.transferAllTasks()
		// TODO: send out retry message
	}
}

// onReceiveMessage on receiving messages
func (w *Worker) onReceiveMessage(from string, msg []byte) (result []byte, err error) {
	var task types.Task
	if err = json.Unmarshal(msg, &task); err != nil {
		_ = w.lg.Log(logger.LevelError, "err", err.Error(), "msg", "failed to unmarshal task message")
		return nil, err
	}
	if _, ok := w.factories[task.Handler]; !ok {
		_ = w.lg.Log(logger.LevelDebug, "taskHandler", task.Handler, "supportHandler", w.factories, "msg", "task not support drop message")
		return []byte("success"), nil
	}
	_ = w.lg.Log(logger.LevelDebug, "running", w.pool.Running(), "capacity", w.pool.Cap(), "taskId", task.Uid,
		"tenantId", task.TenantId, "schedulerId", task.SchedulerId, "handler", task.Handler, "msg", "dispatching task within task pool")
	tc := &types.TaskContext{
		Worker: w.info,
		Task:   task,
	}
	if err = w.run(tc); err != nil {
		_ = w.lg.Log(logger.LevelError, "err", err.Error(), "msg", "failed to start task")
		return nil, err
	}
	return []byte("success"), nil
}

// run pushes a Task into pool and returns immediately
func (w *Worker) stopTask(tc *types.TaskContext) error {
	handler, ok := w.running.Load(tc.Task.Uid)
	if !ok {
		_ = w.lg.Log(logger.LevelInfo, "msg", "revive stop signal but task is not running")
		return nil
	}
	realHandler, ok := handler.(types.TaskHandler)
	if !ok {
		return nil
	}
	return realHandler.Stop()
}

// run pushes a Task into pool and returns immediately
func (w *Worker) run(tc *types.TaskContext) error {
	_, ok := w.running.Load(tc.Task.Uid)
	if ok {
		return fmt.Errorf("task already being executed")
	}
	// Get handler factory and initialize a new handler for the task
	f, ok := w.factories[tc.Task.Handler]
	if !ok {
		return fmt.Errorf("unknown handler %v", tc.Task.Handler)
	}
	hdl, err := f(tc, w.info)
	if err != nil {
		return fmt.Errorf("error creating handler %v for task %v", tc.Task.Handler, tc.Task.Uid)
	}
	// Store handler in the running map
	w.running.Store(tc.Task.Uid, hdl)

	// Start the task
	fn := func() {
		var (
			retry bool
			e     error
		)
		// Notify scheduler that we have started the task
		tc.MarkRunning()
		w.notify(tc.NewMessage(enum.TaskStarted, nil))
		_ = w.lg.Log(logger.LevelInfo, "taskId", tc.Task.Uid, "tenantId", tc.Task.TenantId,
			"schedulerId", tc.Task.SchedulerId, "handler", tc.Task.Handler, "msg", "start to process task")

		defer func() {
			if boom := recover(); boom != nil {
				e = errors.New("task panic")
			}
			status := enum.TaskRunStatusSucceed
			if e != nil {
				status = enum.TaskRunStatusFailed
			}
			tc.MarkDone(status, enum.ResultOmitted, e)
			w.running.Delete(tc.Task.Uid)

			if retry {
				// Task can be retried, notify scheduler to retry this task
				w.notify(tc.NewMessage(enum.RetryTask, nil))
			} else {
				if e != nil {
					w.notify(tc.NewMessage(enum.TaskFailed, nil))
				} else {
					// No retry is needed, notify scheduler task finished
					w.notify(tc.NewMessage(enum.TaskFinished, nil))
				}
			}
		}()

		retry, e = hdl.Start()
		_ = w.lg.Log(logger.LevelInfo, "taskId", tc.Task.Uid, "tenantId", tc.Task.TenantId, "retry", retry,
			"schedulerId", tc.Task.SchedulerId, "handler", tc.Task.Handler, "msg", "finished processing task")
	}
	return w.pool.Submit(fn)
}

// report iterates alive task handlers, collects task status and notifies scheduler about tasks' status
func (w *Worker) report() {
	tick := time.NewTicker(w.opt.ReportInterval)
	for {
		select {
		case <-tick.C:
			w.running.Range(func(k, v interface{}) bool {
				hdl, ok := v.(types.TaskHandler)
				if !ok {
					return true
				}

				tc, s, err := hdl.HeartBeat()
				if err != nil {
					_ = w.lg.Log(logger.LevelError, "err", err.Error(), "msg", "heartbeat error")
					return true
				}
				w.notify(tc.NewMessage(enum.ReportTaskStatus, s))
				return true
			})
		}
	}
}

// notify notifies scheduler about task events through Kafka topic
func (w *Worker) notify(m *types.TaskMessage) {
	byt, err := json.Marshal(m)
	if err != nil {
		_ = w.lg.Log(logger.LevelError, "err", err.Error(), "msg", "failed to marshal message")
		return
	}

	if err = w.tran.Send(w.info.Id, m.SchedulerId, byt); err != nil {
		_ = w.lg.Log(logger.LevelError, "err", err.Error(), "msg", "failed to send message")
	}
}

// transferAllTasks transfers all tasks to other workers
func (w *Worker) transferAllTasks() {
	// TODO: handle task transition properly
	w.running.Range(func(k, v interface{}) bool {
		hdl, ok := v.(types.TaskHandler)
		if !ok {
			return true
		}

		tc, s, err := hdl.TransitionStart()
		if err != nil {
			_ = w.lg.Log(logger.LevelError, "err", err.Error(), "msg", "failed to start task transition")
			w.notify(tc.NewMessage(enum.RetryTask, nil))
			return true
		}
		w.notify(tc.NewMessage(enum.ReportTaskStatus, s))
		return true
	})
}
