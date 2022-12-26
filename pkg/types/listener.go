package types

type ListenerEventMessage struct {
	// tenant id
	TenantUID string
	// task id
	TaskUID string
}

type Listener interface {
	OnTaskCreated(message ListenerEventMessage)

	OnTaskScheduling(message ListenerEventMessage)

	OnTaskDispatching(message ListenerEventMessage)

	OnTaskRunning(message ListenerEventMessage)

	OnTaskNeedRetry(message ListenerEventMessage)

	OnTaskFinished(message ListenerEventMessage)

	OnTaskFail(message ListenerEventMessage)

	OnTaskTransition(message ListenerEventMessage)

	OnTaskTransitionError(message ListenerEventMessage)

	OnTaskTransitionFinished(message ListenerEventMessage)
}
