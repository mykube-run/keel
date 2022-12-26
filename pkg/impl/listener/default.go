package listener

import (
	"github.com/mykube-run/keel/pkg/types"
)

var defaultListener DefaultListener = DefaultListener{}

type DefaultListener struct {
}

func (d DefaultListener) OnTaskScheduling(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskCreated(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskDispatching(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskRunning(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskNeedRetry(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskFinished(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskFail(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskTransition(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskTransitionError(message types.ListenerEventMessage) {
}

func (d DefaultListener) OnTaskTransitionFinished(message types.ListenerEventMessage) {
}
