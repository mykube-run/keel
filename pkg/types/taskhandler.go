package types

// TaskHandlerFactory builds a new TaskHandler
type TaskHandlerFactory func(ctx *TaskContext, info *WorkerInfo) (TaskHandler, error)

// TaskHandler defines the most basic methods that a normal UserTask has to implement.
type TaskHandler interface {
	// Start starts processing the task, returns the error occurred during processing and whether the task should be retried
	// NOTE: This method implementation should be blocking, pooling will be handled by worker
	Start() (bool, error)

	// StartMigratedTask starts a migrated task, signals the worker through finishC when the task has been started
	StartMigratedTask(finishC chan struct{}) (bool, error)

	// Stop forces the task to stop, mostly called when a task is manually stopped
	Stop() error

	// PrepareMigration notifies task handler to pause the processing of tasks, but keep some essential state alive.
	// The task will be started on a new worker after that, so task handler has to take care of this situation.
	PrepareMigration() (*TaskContext, *TaskStatus, error)

	// MigrateError is called when error occurred during migration
	MigrateError() (*TaskContext, *TaskStatus, error)

	// HeartBeat is called by worker regularly to ensure task (and task handler) works as expected.
	// When task handler fails to response for a couple of times, worker may treat the task has been failed
	HeartBeat() (*TaskContext, *TaskStatus, error)
}
