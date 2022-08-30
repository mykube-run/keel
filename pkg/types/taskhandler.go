package types

// TaskHandlerFactory builds a new TaskHandler
type TaskHandlerFactory func(ctx *TaskContext, info *WorkerInfo) (TaskHandler, error)

// TaskHandler defines the most basic methods that a normal UserTask has to implement.
type TaskHandler interface {
	// Start starts a new task, returns the error occurred during processing, and whether the task can be retried
	Start() (bool, error)
	// Stop forces a task to stop, mostly called when a task is manually stopped
	Stop() error
	// HeartBeat is called by worker regularly to ensure task (and task handler) works as expected.
	// When task handler fails to response for a couple of times, worker may treat the task has been failed
	HeartBeat() (*TaskContext, *TaskStatus, error)
	// TransitionStart notifies task handler to pause the processing of tasks, but keep some essential state alive.
	// The task will be started on a new worker after that, so task handler has to take care of this situation.
	TransitionStart() (*TaskContext, *TaskStatus, error)
	// TransitionFinish notifies task handler to stop paused tasks
	TransitionFinish() (*TaskContext, *TaskStatus, error)
	// TransitionError is called when error occurred during transition
	TransitionError() (*TaskContext, *TaskStatus, error)
}
