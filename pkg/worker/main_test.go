package worker

import (
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
	"testing"
	"time"
)

type TestTaskHandler struct {
	ctx *types.TaskContext
}

func TaskHandlerFactory(ctx *types.TaskContext, info *types.WorkerInfo) (types.TaskHandler, error) {
	return &TestTaskHandler{
		ctx: ctx,
	}, nil
}

func (s *TestTaskHandler) Start() (bool, error) {
	log.Info().Str("taskId", s.ctx.Task.Uid).Msg("start to process task")
	time.Sleep(time.Second * 60)
	return false, nil
}

func (s *TestTaskHandler) Stop() error {
	return nil
}

func (s *TestTaskHandler) HeartBeat() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusRunning,
		Progress:  0,
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *TestTaskHandler) TransitionStart() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusNeedsRetry,
		Progress:  0,
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *TestTaskHandler) TransitionFinish() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusNeedsRetry,
		Progress:  0,
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func (s *TestTaskHandler) TransitionError() (*types.TaskContext, *types.TaskStatus, error) {
	status := &types.TaskStatus{
		State:     enum.TaskStatusFailed,
		Progress:  0,
		Error:     nil,
		Timestamp: time.Now(),
	}
	return s.ctx, status, nil
}

func TestKafkaWorker(t *testing.T) {
	name, _ := os.Hostname()
	brokers := strings.Split(os.Getenv("KEEL_KAFKA_BROKERS"), ",")
	opt := &Options{
		PoolSize:       10,
		Name:           name,
		Generation:     0,
		ReportInterval: time.Second * 10,
		Transport: config.TransportConfig{
			Type: "kafka",
			Role: string(enum.TransportRoleWorker),
			Kafka: config.KafkaConfig{
				Brokers: brokers,
				Topics: config.KafkaTopics{
					Tasks:    []string{"cloud-keel-tasks-test-0"},
					Messages: []string{"cloud-keel-messages-test"},
				},
				GroupId:    "cloud-keel-worker-test",
				MessageTTL: 60 * 60,
			},
		},
	}
	lg := zerolog.New(os.Stdout)
	w, err := New(opt, &lg)
	if err != nil {
		t.Fatalf("error creating worker: %v", err)
	}

	w.RegisterHandler("speechFileHandler", TaskHandlerFactory)
	w.Start()
}
