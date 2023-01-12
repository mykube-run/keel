package transport

import (
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/types"
	"strings"
)

func New(conf *config.TransportConfig) (t types.Transport, err error) {
	switch strings.ToLower(conf.Type) {
	case "kafka":
		t, err = NewKafkaTransport(conf)
		return
	default:
		return nil, fmt.Errorf("unsupported transport type: %v", conf.Type)
	}
}
