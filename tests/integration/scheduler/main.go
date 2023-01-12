package main

import (
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/database"
	"github.com/mykube-run/keel/pkg/impl/listener"
	"github.com/mykube-run/keel/pkg/impl/logging"
	"github.com/mykube-run/keel/pkg/scheduler"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	cfg := config.DefaultFromEnv()
	opt := &scheduler.Options{
		Name:                    cfg.Scheduler.Id,
		Zone:                    cfg.Scheduler.Zone,
		ScheduleInterval:        int64(cfg.Scheduler.ScheduleInterval),
		StaleCheckDelay:         int64(cfg.Scheduler.StaleCheckDelay),
		TaskEventUpdateDeadline: int64(cfg.Scheduler.TaskEventUpdateDeadline),
		Snapshot:                cfg.Snapshot,
		Transport:               cfg.Transport,
		ServerConfig: config.ServerConfig{
			HttpAddress: fmt.Sprintf("%v:%v", cfg.Scheduler.Address, cfg.Scheduler.Port),
			GrpcAddress: fmt.Sprintf("%v:%v", cfg.Scheduler.Address, cfg.Scheduler.Port+1000),
		},
	}
	opt.Transport.Role = string(enum.TransportRoleScheduler)

	db, err := database.New(cfg.Database)
	if err != nil {
		log.Fatal().Msgf("error creating database: %v", err)
	}
	s, err := scheduler.New(opt, db, logging.NewDefaultLogger(nil), listener.Default)
	if err != nil {
		log.Fatal().Msgf("error creating scheduler: %v", err)
	}
	s.Start()
}
