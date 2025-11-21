package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

var EnvPrefix = "KEEL_"

func DefaultFromEnv() Config {
	return Config{
		Log: LogConfig{
			Level: str("LOG_LEVEL"),
		},
		Database: DatabaseConfig{
			Type: str("DATABASE_TYPE"),
			DSN:  str("DATABASE_DSN"),
		},
		Scheduler: SchedulerConfig{
			Id:                      envstr("SCHEDULER_ID"),
			Zone:                    envstr("SCHEDULER_ZONE"),
			Port:                    num("SCHEDULER_PORT"),
			Numbers:                 num("SCHEDULER_NUMBERS"),
			Address:                 str("SCHEDULER_ADDRESS"),
			AdvertisedAddress:       str("SCHEDULER_ADVERTISED_ADDRESS"),
			ScheduleInterval:        num("SCHEDULER_SCHEDULE_INTERVAL"),
			StaleCheckDelay:         num("SCHEDULER_STALE_CHECK_DELAY"),
			TaskEventUpdateDeadline: num("SCHEDULER_TASK_EVENT_UPDATE_DEADLINE"),
		},
		Snapshot: SnapshotConfig{
			Enabled:      boolean("SNAPSHOT_ENABLED"),
			MaxVersions:  num("SNAPSHOT_MAX_VERSIONS"),
			Interval:     time.Second * time.Duration(num("SNAPSHOT_INTERVAL")),
			Endpoint:     str("SNAPSHOT_ENDPOINT"),
			Region:       str("SNAPSHOT_REGION"),
			Bucket:       str("SNAPSHOT_BUCKET"),
			AccessKey:    str("SNAPSHOT_ACCESS_KEY"),
			AccessSecret: str("SNAPSHOT_ACCESS_SECRET"),
			Secure:       boolean("SNAPSHOT_SECURE"),
		},
		Worker: WorkerConfig{
			Name:           envstr("WORKER_NAME"),
			PoolSize:       num("WORKER_POOL_SIZE"),
			Generation:     num("WORKER_GENERATION"),
			ReportInterval: num("WORKER_REPORT_INTERVAL"),
		},
		Transport: TransportConfig{
			Type: str("TRANSPORT_TYPE"),
			Role: str("TRANSPORT_ROLE"),
			Kafka: KafkaConfig{
				Brokers: strs("TRANSPORT_KAFKA_BROKERS"),
				Topics: KafkaTopics{
					Tasks:    strs("TRANSPORT_KAFKA_TOPICS_TASKS"),
					Messages: strs("TRANSPORT_KAFKA_TOPICS_MESSAGES"),
				},
				GroupId:      envstr("TRANSPORT_KAFKA_GROUP_ID"),
				MessageTTL:   num("TRANSPORT_KAFKA_MESSAGE_TTL"),
				EnableSASL:   boolean("TRANSPORT_KAFKA_ENABLE_SASL"),
				SASLUsername: str("TRANSPORT_KAFKA_SASL_USERNAME"),
				SASLPassword: str("TRANSPORT_KAFKA_SASL_PASSWORD"),
			},
			Grpc: GrpcConfig{
				Mode:               str("TRANSPORT_GRPC_MODE"),
				SchedulerEndpoints: strs("TRANSPORT_GRPC_SCHEDULER_ENDPOINTS"),
				ListenAddress:      str("TRANSPORT_GRPC_LISTEN_ADDRESS"),
				APIKey:             str("TRANSPORT_GRPC_API_KEY"),
				DNSName:            str("TRANSPORT_GRPC_DNS_NAME"),
				K8SNamespace:       str("TRANSPORT_GRPC_K8S_NAMESPACE"),
				K8SService:         str("TRANSPORT_GRPC_K8S_SERVICE"),
				TLSEnable:          boolean("TRANSPORT_GRPC_TLS_ENABLE"),
				TLSCAFile:          str("TRANSPORT_GRPC_TLS_CA_FILE"),
				TLSCertFile:        str("TRANSPORT_GRPC_TLS_CERT_FILE"),
				TLSKeyFile:         str("TRANSPORT_GRPC_TLS_KEY_FILE"),
				InsecureSkipVerify: boolean("TRANSPORT_GRPC_TLS_INSECURE_SKIP_VERIFY"),
				SendRetryMax:             num("TRANSPORT_GRPC_SEND_RETRY_MAX"),
				SendRetryInitialBackoffMs: num("TRANSPORT_GRPC_SEND_RETRY_INITIAL_BACKOFF_MS"),
				SendRetryMaxBackoffMs:     num("TRANSPORT_GRPC_SEND_RETRY_MAX_BACKOFF_MS"),
				SendRetryJitterPct:        num("TRANSPORT_GRPC_SEND_RETRY_JITTER_PCT"),
				ReconnectOnSendError:      boolean("TRANSPORT_GRPC_RECONNECT_ON_SEND_ERROR"),
				Auth: AuthConfig{
					Type:    str("TRANSPORT_GRPC_AUTH_TYPE"),
					APIKeys: strs("TRANSPORT_GRPC_AUTH_API_KEYS"),
				},
			},
		},
	}
}

func ReplaceEnvironment(val string) string {
	hostname, _ := os.Hostname()
	podname := os.Getenv("POD_NAME")
	pid := strconv.Itoa(os.Getpid())
	val = strings.ReplaceAll(val, "{hostname}", hostname)
	val = strings.ReplaceAll(val, "{podname}", podname)
	val = strings.ReplaceAll(val, "{pid}", pid)
	return val
}

func str(key string) string {
	return strings.TrimSpace(os.Getenv(EnvPrefix + key))
}

func strs(key string) []string {
	val := strings.Split(os.Getenv(EnvPrefix+key), ",")
	for i, _ := range val {
		val[i] = strings.TrimSpace(val[i])
	}
	return val
}

func boolean(key string) bool {
	val := strings.ToLower(str(key))
	return val == "true" || val == "1"
}

func num(key string) int {
	val, _ := strconv.Atoi(str(key))
	return val
}

func envstr(key string) string {
	return ReplaceEnvironment(str(key))
}
