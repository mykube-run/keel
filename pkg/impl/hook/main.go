package hook

import (
	"github.com/mykube-run/keel/pkg/types"
)

// Default is the global default hook
var Default = new(defaultHook)

type defaultHook struct {
}

func (d *defaultHook) OnTaskScheduling(e types.HookEvent) {
}

func (d *defaultHook) OnTaskCreated(e types.HookEvent) {
}

func (d *defaultHook) OnTaskDispatching(e types.HookEvent) {
}

func (d *defaultHook) OnTaskRunning(e types.HookEvent) {
}

func (d *defaultHook) OnTaskNeedsRetry(e types.HookEvent) {
}

func (d *defaultHook) OnTaskFinished(e types.HookEvent) {
}

func (d *defaultHook) OnTaskFailed(e types.HookEvent) {
}

func (d *defaultHook) OnTaskMigrationStart(e types.HookEvent) {
}

func (d *defaultHook) OnTaskMigrationError(e types.HookEvent) {
}

func (d *defaultHook) OnTaskMigrationFinished(e types.HookEvent) {
}
