package worker

import (
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/transport"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/panjf2000/ants/v2"
	"os"
	"os/signal"
	"runtime"
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
	lg        types.Logger
	opt       *Options
	pool      *ants.Pool
	tran      types.Transport
	info      *types.WorkerInfo
	running   sync.Map
	factories map[string]types.TaskHandlerFactory
}

// New initializes a new Worker
func New(opt *Options, lg types.Logger) (*Worker, error) {
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
	w.lg.Log(types.LevelInfo, "handlers", w.handlers(),
		"workerId", w.opt.Name, "poolSize", w.opt.PoolSize, "message", "starting worker")
	go w.report()

	stopC := make(chan os.Signal)
	signal.Notify(stopC, os.Interrupt, syscall.SIGTERM /* SIGTERM is expected inside k8s */)

	select {
	case <-stopC:
		w.lg.Log(types.LevelInfo, "message", "received stop signal")
		_ = w.tran.CloseReceiving()
		time.Sleep(500 * time.Millisecond)
		w.transferAllTasks()
		_ = w.tran.CloseSend()
	}
}

// onReceiveMessage on receiving messages
func (w *Worker) onReceiveMessage(from string, msg []byte) (result []byte, err error) {
	var task types.Task
	if err = json.Unmarshal(msg, &task); err != nil {
		w.lg.Log(types.LevelError, "error", err.Error(), "message", "failed to unmarshal task message")
		return nil, err
	}
	if _, ok := w.factories[task.Handler]; !ok {
		return []byte(task.Handler), fmt.Errorf("unsupported handler")
	}

	w.lg.Log(types.LevelDebug, "running", w.pool.Running(), "capacity", w.pool.Cap(), "taskId", task.Uid,
		"tenantId", task.TenantId, "schedulerId", task.SchedulerId, "handler", task.Handler,
		"message", "dispatching task within worker pool")
	tc := &types.TaskContext{
		Worker: w.info,
		Task:   task,
	}
	if err = w.run(tc); err != nil {
		w.lg.Log(types.LevelError, "error", err.Error(), "message", "failed to start task")
		return nil, err
	}
	return []byte("success"), nil
}

// run stops the Task
func (w *Worker) stopTask(tc *types.TaskContext) error {
	tmp, ok := w.running.Load(tc.Task.Uid)
	if !ok {
		w.lg.Log(types.LevelInfo, "message", "received stop signal but task is not running")
		return nil
	}
	hdl, ok := tmp.(types.TaskHandler)
	if !ok {
		return nil
	}
	return hdl.Stop()
}

// run pushes a Task into pool and returns immediately
func (w *Worker) run(tc *types.TaskContext) error {
	_, ok := w.running.Load(tc.Task.Uid)
	if ok {
		return fmt.Errorf("task already processing")
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

		defer func() {
			if r := recover(); r != nil {
				w.printStack(tc.Task, r)
				e = fmt.Errorf("task handler panicked: %v", r)
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

		// if task is in transition, start transition
		if tc.Task.InTransition {
			w.notify(tc.NewMessage(enum.FinishTransition, nil))
			w.lg.Log(types.LevelInfo, "taskId", tc.Task.Uid, "tenantId", tc.Task.TenantId,
				"schedulerId", tc.Task.SchedulerId, "handler", tc.Task.Handler,
				"message", "start processing transited task")
			retry, e = hdl.Start()
		} else {
			// Notify scheduler that we have started the task
			tc.MarkRunning()
			w.notify(tc.NewMessage(enum.TaskStarted, nil))
			w.lg.Log(types.LevelInfo, "taskId", tc.Task.Uid, "tenantId", tc.Task.TenantId,
				"schedulerId", tc.Task.SchedulerId, "handler", tc.Task.Handler,
				"message", "start processing task")
			retry, e = hdl.Start()
		}
		w.lg.Log(types.LevelInfo, "taskId", tc.Task.Uid, "tenantId", tc.Task.TenantId, "retry", retry,
			"schedulerId", tc.Task.SchedulerId, "handler", tc.Task.Handler,
			"message", "finished processing task")
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
					w.lg.Log(types.LevelError, "error", err.Error(), "message", "heartbeat error")
					return true
				}
				w.notify(tc.NewMessage(enum.ReportTaskStatus, s))
				return true
			})
		}
	}
}

// notify scheduler about task events through Kafka topic
func (w *Worker) notify(m *types.TaskMessage) {
	byt, err := json.Marshal(m)
	if err != nil {
		w.lg.Log(types.LevelError, "error", err.Error(), "message", "failed to marshal message")
		return
	}

	if err = w.tran.Send(w.info.Id, m.SchedulerId, byt); err != nil {
		w.lg.Log(types.LevelError, "error", err.Error(),
			"workerId", w.info.Id, "schedulerId", m.SchedulerId,
			"message", "failed to send message")
	}
}

// transferAllTasks transfers all tasks to other workers
func (w *Worker) transferAllTasks() {
	w.running.Range(func(k, v interface{}) bool {
		hdl, ok := v.(types.TaskHandler)
		if !ok {
			return true
		}

		tc, s, err := hdl.BeforeTransitionStart()
		if err != nil {
			w.lg.Log(types.LevelError, "error", err.Error(), "taskId", tc.Task.Uid, "tenantId", tc.Task.TenantId,
				"message", "failed to start task transition")
			w.notify(tc.NewMessage(enum.RetryTask, nil))
			return true
		}
		w.notify(tc.NewMessage(enum.StartTransition, s))
		w.lg.Log(types.LevelWarn, "taskId", tc.Task.Uid, "tenantId", tc.Task.TenantId,
			"message", "succeeded starting task transition")
		return true
	})
}

// handlers returns supported handler names
func (w *Worker) handlers() []string {
	hdl := make([]string, 0)
	for k := range w.factories {
		hdl = append(hdl, k)
	}
	return hdl
}

// printStack logs exception stack
func (w *Worker) printStack(t types.Task, err interface{}) {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	w.lg.Log(types.LevelError, "taskId", t.Uid,
		"tenantId", t.TenantId, "error", err, "stack", string(buf[:n]),
		"message", "task handler panicked")
}
