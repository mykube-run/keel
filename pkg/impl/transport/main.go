package transport

import (
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/types"
)

func New(cfg *config.TransportConfig) (t types.Transport, err error) {
	switch cfg.Type {
	case "kafka":
		t, err = NewKafkaTransport(cfg)
		return
	default:
		return nil, fmt.Errorf("unsupported transport type: %v", cfg.Type)
	}
}
