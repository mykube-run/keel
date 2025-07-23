package handlers

import (
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog/log"
	"time"
)

const HangUpDuration = 60

type HangUpTaskHandler struct {
	started time.Time
	ctx     *types.TaskContext
}

func HangUpTaskHandlerFactory(ctx *types.TaskContext, info *types.WorkerInfo) (types.TaskHandler, error) {
	return &HangUpTaskHandler{
		ctx: ctx,
	}, nil
}

func (s *HangUpTaskHandler) Start() (bool, error) {
	s.started = time.Now()
	log.Info().Str("taskId", s.ctx.Task.Uid).
		Msgf("start to process task, will hang up for %v seconds and return error on HeartBeat call", HangUpDuration)
	time.Sleep(time.Second * HangUpDuration)
	return false, nil
}

func (s *HangUpTaskHandler) HeartBeat() (*types.TaskContext, *types.TaskStatus, error) {
	return s.ctx, nil, nil
}

func (s *HangUpTaskHandler) StartMigratedTask(chan struct{}) (bool, error) {
	s.started = time.Now()
	log.Info().Str("taskId", s.ctx.Task.Uid).
		Msgf("start to process task, will hang up for %v seconds and return error on HeartBeat call", HangUpDuration)
	time.Sleep(time.Second * HangUpDuration)
	return false, nil
}

func (s *HangUpTaskHandler) Stop() error {
	return nil
}

func (s *HangUpTaskHandler) PrepareMigration() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusNeedsRetry,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *HangUpTaskHandler) TransitionFinish() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusNeedsRetry,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *HangUpTaskHandler) MigrateError() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusFailed,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *HangUpTaskHandler) progress() int {
	dur := time.Now().Sub(s.started).Seconds()
	p := int(dur / float64(OrdinaryTaskDuration) * 100)
	if p > 100 {
		p = 100
	}
	return p
}

//func (s *HangUpTaskHandler) NotifyTransitionFinish(signalChan chan struct{}) {
//	signalChan <- struct{}{}
//}
