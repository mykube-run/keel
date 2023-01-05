package scheduler

import (
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/database"
	"github.com/mykube-run/keel/pkg/impl/listener"
	"github.com/mykube-run/keel/pkg/impl/logging"
	"github.com/mykube-run/keel/pkg/scheduler"
	"github.com/mykube-run/keel/tests/testkit/worker"
	"github.com/rs/zerolog"
	"testing"
)

func Test_Main(t *testing.T) {
	go worker.StartTestWorkers()

	cfg := config.DefaultFromEnv()
	cfg.Transport.Role = string(enum.TransportRoleScheduler)
	opt := &scheduler.Options{
		Name:             cfg.Scheduler.Id,
		Zone:             cfg.Scheduler.Zone,
		ScheduleInterval: int64(cfg.Scheduler.ScheduleInterval),
		StaleCheckDelay:  int64(cfg.Scheduler.StaleCheckDelay),
		Snapshot:         cfg.Snapshot,
		Transport:        cfg.Transport,
	}
	db := database.NewMockDB()
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	s, err := scheduler.New(opt, db, logging.NewDefaultLogger(nil), listener.Default)
	if err != nil {
		t.Fatalf("error creating scheduler: %v", err)
	}
	s.Start()
}
