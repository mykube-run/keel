package workers

import (
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog/log"
	"time"
)

const NormalTaskDuration = 120

type NormalTaskHandler struct {
	started time.Time
	ctx     *types.TaskContext
}

func NormalTaskHandlerFactory(ctx *types.TaskContext, info *types.WorkerInfo) (types.TaskHandler, error) {
	return &NormalTaskHandler{
		ctx: ctx,
	}, nil
}

func (s *NormalTaskHandler) Start() (bool, error) {
	s.started = time.Now()
	log.Info().Str("taskId", s.ctx.Task.Uid).
		Msgf("start to process task, will sleep for %v seconds and finish the task", NormalTaskDuration)
	time.Sleep(time.Second * NormalTaskDuration)
	return false, nil
}

func (s *NormalTaskHandler) Stop() error {
	return nil
}

func (s *NormalTaskHandler) HeartBeat() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusRunning,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *NormalTaskHandler) TransitionStart() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusNeedsRetry,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *NormalTaskHandler) TransitionFinish() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusNeedsRetry,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *NormalTaskHandler) TransitionError() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusFailed,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *NormalTaskHandler) progress() int {
	dur := time.Now().Sub(s.started).Seconds()
	p := int(dur / float64(NormalTaskDuration) * 100)
	if p > 100 {
		p = 100
	}
	return p
}
