package worker

import (
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/mykube-run/keel/pkg/worker"
	"github.com/mykube-run/keel/tests/testkit/worker/workers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/errgo.v2/errors"
	"strings"
	"time"
)

type DefaultLogger struct {
	lg zerolog.Logger
}

func (k DefaultLogger) Log(logLevel types.Level, keyvals ...interface{}) error {
	if len(keyvals)%2 != 0 {
		return errors.New("key-value cannot be paired")
	}
	pairNumber := len(keyvals) / 2
	level, err := zerolog.ParseLevel(strings.ToLower(logLevel.String()))
	if err != nil {
		return err
	}
	var event *zerolog.Event
	event = k.lg.WithLevel(level)
	for i := 0; i < pairNumber; i++ {
		keyString, ok := keyvals[i*2].(string)
		if !ok {
			return fmt.Errorf("key must be string")
		}
		value := keyvals[i*2+1]
		event = event.Interface(keyString, value)
	}
	event.Send()
	return nil
}

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
	dl := DefaultLogger{lg: zerolog.Logger{}}
	w, err := worker.New(opt, &dl)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating worker")
	}

	w.RegisterHandler("normal", workers.NormalTaskHandlerFactory)
	w.RegisterHandler("retry", workers.RetryTaskHandlerFactory)
	w.RegisterHandler("hangup", workers.HangUpTaskHandlerFactory)
	w.Start()
}
