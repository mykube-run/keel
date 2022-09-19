package worker

import (
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/worker"
	"github.com/mykube-run/keel/tests/testkit/worker/workers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"time"
)

func StartTestWorkers() {
	cfg := config.DefaultFromEnv()
	cfg.Transport.Role = string(enum.TransportRoleWorker)
	opt := &worker.Options{
		PoolSize:       cfg.Worker.PoolSize,
		Name:           cfg.Worker.Name,
		Generation:     int64(cfg.Worker.Generation),
		ReportInterval: time.Second * time.Duration(cfg.Worker.ReportInterval),
		Transport:      cfg.Transport,
	}
	lg := zerolog.New(os.Stdout)
	w, err := worker.New(opt, &lg)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating worker")
	}

	w.RegisterHandler("normal", workers.NormalTaskHandlerFactory)
	w.RegisterHandler("retry", workers.RetryTaskHandlerFactory)
	w.RegisterHandler("hangup", workers.HangUpTaskHandlerFactory)
	w.Start()
}