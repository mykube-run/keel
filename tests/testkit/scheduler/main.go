package scheduler

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

func StartScheduler() {
	cfg := config.DefaultFromEnv()
	cfg.Transport.Role = string(enum.TransportRoleScheduler)
	opt := &scheduler.Options{
		Name:             cfg.Scheduler.Id,
		Zone:             cfg.Scheduler.Zone,
		ScheduleInterval: int64(cfg.Scheduler.ScheduleInterval),
		StaleCheckDelay:  int64(cfg.Scheduler.StaleCheckDelay),
		Snapshot:         cfg.Snapshot,
		Transport:        cfg.Transport,
		ServerConfig: config.ServerConfig{
			HttpAddress: fmt.Sprintf("%v:%v", cfg.Scheduler.Address, cfg.Scheduler.Port),
			GrpcAddress: fmt.Sprintf("%v:%v", cfg.Scheduler.Address, cfg.Scheduler.Port+1000),
		},
	}
	db, err := database.New(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating database")
	}
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	s, err := scheduler.New(opt, db, logging.NewDefaultLogger(nil), listener.Default)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating scheduler")
	}
	s.Start()
}
