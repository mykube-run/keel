package transport

import (
	"fmt"
	"strings"

	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
)

const apiKeyHeader = "x-api-key"
const identifierHeader = "x-identifier"
const workerHandlersHeader = "x-worker-handlers"
const heartbeatTopic = "__heartbeat"

func NewGrpcTransport(cfg *config.TransportConfig) (types.Transport, error) {
	if err := validateGrpcConfig(cfg); err != nil {
		return nil, err
	}
	if strings.EqualFold(cfg.Role, string(enum.TransportRoleScheduler)) {
		return newGrpcSchedulerTransport(cfg)
	}
	return newGrpcWorkerTransport(cfg)
}

func validateGrpcConfig(cfg *config.TransportConfig) error {
	if strings.TrimSpace(cfg.Identifier) == "" {
		return fmt.Errorf("TransportConfig.Identifier was not specified")
	}
	if cfg.Role != string(enum.TransportRoleScheduler) && cfg.Role != string(enum.TransportRoleWorker) {
		return fmt.Errorf("TransportConfig.Role was not specified")
	}
	if cfg.Role == string(enum.TransportRoleScheduler) {
		if strings.TrimSpace(cfg.Grpc.ListenAddress) == "" {
			return fmt.Errorf("TransportConfig.Grpc.ListenAddress was not specified")
		}
	} else {
		if strings.TrimSpace(cfg.Grpc.Mode) == "" {
			return fmt.Errorf("TransportConfig.Grpc.Mode was not specified")
		}
		if cfg.Grpc.Mode == "static" && len(cfg.Grpc.SchedulerEndpoints) == 0 {
			return fmt.Errorf("TransportConfig.Grpc.SchedulerEndpoints was not specified")
		}
		if cfg.Grpc.Mode == "dns" && strings.TrimSpace(cfg.Grpc.DNSName) == "" {
			return fmt.Errorf("TransportConfig.Grpc.DNSName was not specified")
		}
		if cfg.Grpc.Mode == "k8s" && (strings.TrimSpace(cfg.Grpc.K8SNamespace) == "" || strings.TrimSpace(cfg.Grpc.K8SService) == "") {
			return fmt.Errorf("TransportConfig.Grpc.K8S fields were not specified")
		}
	}
	if strings.TrimSpace(cfg.Grpc.APIKey) == "" {
		return fmt.Errorf("TransportConfig.Grpc.APIKey was not specified")
	}
	return nil
}
