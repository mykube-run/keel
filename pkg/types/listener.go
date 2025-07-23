package types

type ListenerEvent struct {
	SchedulerId string
	WorkerId    string
	Task        TaskMetadata
}

type Listener interface {
	OnTaskCreated(e ListenerEvent)
	OnTaskScheduling(e ListenerEvent)
	OnTaskDispatching(e ListenerEvent)
	OnTaskRunning(e ListenerEvent)
	OnTaskNeedsRetry(e ListenerEvent)
	OnTaskFinished(e ListenerEvent)
	OnTaskFailed(e ListenerEvent)
	OnTaskTransition(e ListenerEvent)
	OnTaskTransitionError(e ListenerEvent)
	OnTaskTransitionFinished(e ListenerEvent)
	OnStaleTaskRetry(e ListenerEvent)
}
