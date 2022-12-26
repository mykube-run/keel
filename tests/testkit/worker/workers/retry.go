package workers

import (
	"fmt"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog/log"
	"math/rand"
	"time"
)

const (
	NumOfRetries = 2
)

type RetryTaskHandler struct {
	started time.Time
	ctx     *types.TaskContext
}

func RetryTaskHandlerFactory(ctx *types.TaskContext, info *types.WorkerInfo) (types.TaskHandler, error) {
	return &RetryTaskHandler{
		ctx: ctx,
	}, nil
}

func (s *RetryTaskHandler) HeartBeat() (*types.TaskContext, *types.TaskStatus, error) {
	return nil, nil, nil
}

func (s *RetryTaskHandler) Start() (bool, error) {
	s.started = time.Now()
	log.Info().Str("taskId", s.ctx.Task.Uid).
		Msgf("start to process task, will sleep for at most %v seconds, retry %v times and finish the task",
			NormalTaskDuration, NumOfRetries)
	if s.ctx.Task.RestartTimes <= 2 {
		time.Sleep(time.Second * time.Duration(rand.Int63n(60)+30))
		return true, fmt.Errorf("task needs retry")
	}
	time.Sleep(time.Second * NormalTaskDuration)
	return false, nil
}

func (s *RetryTaskHandler) StartTransitionTask(chan struct{}) (bool, error) {
	s.started = time.Now()
	log.Info().Str("taskId", s.ctx.Task.Uid).
		Msgf("start to process task, will sleep for at most %v seconds, retry %v times and finish the task",
			NormalTaskDuration, NumOfRetries)
	if s.ctx.Task.RestartTimes <= 2 {
		time.Sleep(time.Second * time.Duration(rand.Int63n(60)+30))
		return true, fmt.Errorf("task needs retry")
	}
	time.Sleep(time.Second * NormalTaskDuration)
	return false, nil
}

func (s *RetryTaskHandler) Stop() error {
	return nil
}

func (s *RetryTaskHandler) BeforeTransitionStart() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusNeedsRetry,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *RetryTaskHandler) TransitionFinish() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusNeedsRetry,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *RetryTaskHandler) TransitionError() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusFailed,
		Progress:  s.progress(),
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *RetryTaskHandler) progress() int {
	dur := time.Now().Sub(s.started).Seconds()
	p := int(dur / 60.0 * 100)
	if p > 100 {
		p = 100
	}
	return p
}

func (s *RetryTaskHandler) NotifyTransitionFinish(signalChan chan struct{}) {
	signalChan <- struct{}{}
}
