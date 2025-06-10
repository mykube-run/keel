package handlers

import (
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog/log"
	"time"
)

const OrdinaryTaskDuration = 120

type OrdinaryTaskHandler struct {
	started time.Time
	ctx     *types.TaskContext
}

func OrdinaryTaskHandlerFactory(ctx *types.TaskContext, info *types.WorkerInfo) (types.TaskHandler, error) {
	return &OrdinaryTaskHandler{
		ctx: ctx,
	}, nil
}

func (s *OrdinaryTaskHandler) Start() (bool, error) {
	s.started = time.Now()
	log.Info().Str("taskId", s.ctx.Task.Uid).
		Msgf("start to process task, will sleep for %v seconds and finish the task", OrdinaryTaskDuration)
	time.Sleep(time.Second * OrdinaryTaskDuration)
	return false, nil
}

func (s *OrdinaryTaskHandler) HeartBeat() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusRunning,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *OrdinaryTaskHandler) PrepareMigration() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusMigrating,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *OrdinaryTaskHandler) StartMigratedTask(chan struct{}) (bool, error) {
	return false, nil
}

func (s *OrdinaryTaskHandler) TransitionFinish() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusMigrating,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *OrdinaryTaskHandler) MigrateError() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusFailed,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *OrdinaryTaskHandler) Stop() error {
	return nil
}

func (s *OrdinaryTaskHandler) progress() int {
	dur := time.Now().Sub(s.started).Seconds()
	p := int(dur / float64(OrdinaryTaskDuration) * 100)
	if p > 100 {
		p = 100
	}
	return p
}
