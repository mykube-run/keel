package scheduler

//func Test_Main(t *testing.T) {
//	go worker.StartTestWorkers()
//
//	cfg := config.DefaultFromEnv()
//	cfg.Transport.Role = string(enum.TransportRoleScheduler)
//	opt := &scheduler.Options{
//		Name:             cfg.Scheduler.Id,
//		Zone:             cfg.Scheduler.Zone,
//		ScheduleInterval: int64(cfg.Scheduler.ScheduleInterval),
//		StaleCheckDelay:  int64(cfg.Scheduler.StaleCheckDelay),
//		Snapshot:         cfg.Snapshot,
//		Transport:        cfg.Transport,
//	}
//	db := database.NewMockDB()
//	lg := zerolog.New(os.Stdout)
//	zerolog.SetGlobalLevel(zerolog.TraceLevel)
//	s, err := scheduler.New(opt, db, &lg)
//	if err != nil {
//		t.Fatalf("error creating scheduler: %v", err)
//	}
//	s.Start()
//}
