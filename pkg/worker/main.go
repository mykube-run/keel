package worker

import (
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/transport"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog"
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
	lg        *zerolog.Logger
	opt       *Options
	pool      *ants.Pool
	tran      types.Transport
	info      *types.WorkerInfo
	running   sync.Map
	factories map[string]types.TaskHandlerFactory
}

// New initializes a new Worker
func New(opt *Options, lg *zerolog.Logger) (*Worker, error) {
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
	w.lg.Info().Str("workerId", w.opt.Name).Int("poolSize", w.opt.PoolSize).Msg("starting worker")
	go w.report()

	stopC := make(chan os.Signal)
	signal.Notify(stopC, os.Interrupt, syscall.SIGTERM /* SIGTERM is expected inside k8s */)

	select {
	case <-stopC:
		w.lg.Info().Msg("received stop signal")
		_ = w.tran.Close()
		w.transferAllTasks()
		// TODO: send out retry message
	}
}

// onReceiveMessage on receiving messages
func (w *Worker) onReceiveMessage(from string, msg []byte) (result []byte, err error) {
	var task types.Task
	if err = json.Unmarshal(msg, &task); err != nil {
		w.lg.Err(err).Msg("failed to unmarshal task message")
		return nil, err
	}

	w.lg.Debug().Int("running", w.pool.Running()).Int("capacity", w.pool.Cap()).
		Str("taskId", task.Uid).Str("tenantId", task.TenantId).
		Str("schedulerId", task.SchedulerId).Str("handler", task.Handler).
		Msg("dispatching task within task pool")

	tc := &types.TaskContext{
		Worker: w.info,
		Task:   task,
	}
	if err = w.run(tc); err != nil {
		w.lg.Err(err).Msg("failed to start task")
		return nil, err
	}
	return []byte("success"), nil
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
		w.lg.Info().Str("taskId", tc.Task.Uid).Str("tenantId", tc.Task.TenantId).
			Str("schedulerId", tc.Task.SchedulerId).Str("handler", tc.Task.Handler).
			Msg("start to process task")

		defer func() {
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
				// No retry is needed, notify scheduler task finished
				w.notify(tc.NewMessage(enum.TaskFinished, nil))
			}
		}()

		retry, e = hdl.Start()
		w.lg.Info().Str("taskId", tc.Task.Uid).Str("tenantId", tc.Task.TenantId).
			Str("schedulerId", tc.Task.SchedulerId).Str("handler", tc.Task.Handler).
			Err(e).Bool("retry", retry).Msg("finished processing task")
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
					w.lg.Err(err).Msg("heartbeat error")
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
		w.lg.Err(err).Msg("failed to marshal message")
		return
	}

	if err = w.tran.Send(w.info.Id, m.SchedulerId, byt); err != nil {
		w.lg.Err(err).Msg("failed to send message")
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
			w.lg.Err(err).Msg("failed to start task transition")
			w.notify(tc.NewMessage(enum.RetryTask, nil))
			return true
		}
		w.notify(tc.NewMessage(enum.ReportTaskStatus, s))
		return true
	})
}
