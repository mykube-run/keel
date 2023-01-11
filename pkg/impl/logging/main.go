package logging

import (
	"fmt"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog"
	"os"
	"strings"
)

type defaultLogger struct {
	lg *zerolog.Logger
}

func NewDefaultLogger(lg *zerolog.Logger) types.Logger {
	if lg == nil {
		tmp := zerolog.New(os.Stdout).With().Timestamp().Logger()
		return &defaultLogger{lg: &tmp}
	}
	return &defaultLogger{lg: lg}
}

func (d *defaultLogger) Log(lvl types.Level, keyvals ...interface{}) {
	if len(keyvals)%2 != 0 {
		d.lg.Warn().Str("keyvals", fmt.Sprintf("%+v", keyvals)).Msg("key-value cannot be paired")
		return
	}

	pairs := len(keyvals) / 2
	level, err := zerolog.ParseLevel(strings.ToLower(lvl.String()))
	if err != nil {
		level = zerolog.InfoLevel
	}

	event := d.lg.WithLevel(level)
	for i := 0; i < pairs; i++ {
		key, ok := keyvals[i*2].(string)
		if !ok {
			d.lg.Warn().Str("keyvals", fmt.Sprintf("%+v", keyvals)).Msg("key must be string")
			return
		}
		val := keyvals[i*2+1]
		event = event.Interface(key, val)
	}
	event.Send()
}
