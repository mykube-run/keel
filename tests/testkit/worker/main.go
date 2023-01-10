package worker

import (
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/logging"
	"github.com/mykube-run/keel/pkg/worker"
	"github.com/mykube-run/keel/tests/testkit/worker/workers"
	"testing"
	"time"
)

func StartTestWorkers(t *testing.T) {
	cfg := config.DefaultFromEnv()
	if cfg.Transport.Type == "kafka" {
		cfg.Transport.Role = string(enum.TransportRoleWorker)
	}
	opt := &worker.Options{
		PoolSize:       cfg.Worker.PoolSize,
		Name:           cfg.Worker.Name,
		Generation:     int64(cfg.Worker.Generation),
		ReportInterval: time.Second * time.Duration(cfg.Worker.ReportInterval),
		Transport:      cfg.Transport,
	}
	w, err := worker.New(opt, logging.NewDefaultLogger(nil))
	if err != nil {
		t.Fatalf("error creating worker: %v", err)
	}

	w.RegisterHandler("ordinary", workers.OrdinaryTaskHandlerFactory)
	w.RegisterHandler("retry", workers.RetryTaskHandlerFactory)
	w.RegisterHandler("hangup", workers.HangUpTaskHandlerFactory)
	w.Start()
}
