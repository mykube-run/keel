package transport

import (
	"fmt"
	"strings"

	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/types"
)

func New(conf *config.TransportConfig) (t types.Transport, err error) {
	switch strings.ToLower(conf.Type) {
	case "kafka":
		t, err = NewKafkaTransport(conf)
		return
	case "grpc":
		t, err = NewGrpcTransport(conf)
		return
	default:
		return nil, fmt.Errorf("unsupported transport type: %v", conf.Type)
	}
}

func SetWorkerHandlers(t types.Transport, handlers []string) {
	if gw, ok := t.(*GrpcWorkerTransport); ok {
		gw.setHandlers(handlers)
	}
}
