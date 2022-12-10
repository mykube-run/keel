package types

type ListenerEventMessage struct {
	//tenant info
	TenantUID string
	//TaskUID
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
}