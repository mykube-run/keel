package types

type HookEvent struct {
	SchedulerId string
	WorkerId    string
	Task        TaskMetadata
}

type Hooks interface {
	OnTaskCreated(e HookEvent)
	OnTaskScheduling(e HookEvent)
	OnTaskDispatching(e HookEvent)
	OnTaskRunning(e HookEvent)
	OnTaskNeedsRetry(e HookEvent)
	OnTaskFinished(e HookEvent)
	OnTaskFailed(e HookEvent)
	OnTaskMigrationStart(e HookEvent)
	OnTaskMigrationError(e HookEvent)
	OnTaskMigrationFinished(e HookEvent)
}
