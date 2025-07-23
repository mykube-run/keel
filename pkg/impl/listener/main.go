package listener

import (
	"github.com/mykube-run/keel/pkg/types"
)

// Default is the global default listener
var Default = new(defaultListener)

type defaultListener struct {
}

func (d *defaultListener) OnTaskScheduling(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskCreated(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskDispatching(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskRunning(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskNeedsRetry(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskFinished(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskFailed(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskTransition(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskTransitionError(e types.ListenerEvent) {
}

func (d *defaultListener) OnTaskTransitionFinished(e types.ListenerEvent) {
}

func (d *defaultListener) OnStaleTaskRetry(e types.ListenerEvent) {
}
