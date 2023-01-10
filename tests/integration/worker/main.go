package main

import (
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/logging"
	"github.com/mykube-run/keel/pkg/worker"
	"github.com/mykube-run/keel/tests/integration/worker/handlers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"time"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
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
		log.Fatal().Msgf("error creating worker: %v", err)
	}

	w.RegisterHandler("ordinary", handlers.OrdinaryTaskHandlerFactory)
	w.RegisterHandler("retry", handlers.RetryTaskHandlerFactory)
	w.RegisterHandler("hangup", handlers.HangUpTaskHandlerFactory)
	w.Start()
}
