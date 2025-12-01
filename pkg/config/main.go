package config

import (
	"time"

	"github.com/rs/zerolog"
)

type Config struct {
	Log       LogConfig
	Database  DatabaseConfig
	Scheduler SchedulerConfig
	Snapshot  SnapshotConfig
	Worker    WorkerConfig
	Transport TransportConfig
}

type LogConfig struct {
	Level string
}

func (lc *LogConfig) GetLevel() zerolog.Level {
	lvl, err := zerolog.ParseLevel(lc.Level)
	if err == nil {
		return lvl
	}
	return zerolog.DebugLevel
}

type DatabaseConfig struct {
	Type string
	DSN  string
}

type SchedulerConfig struct {
	Name                    string // Scheduler name, also used to identify partition
	Zone                    string // The zone to which schedule belongs to
	Port                    int
	Numbers                 int // Number of schedulers within the same zone
	Address                 string
	AdvertisedAddress       string
	ScheduleInterval        int // Schedule interval in seconds
	StaleCheckDelay         int // Stale tasks check delay after start up in seconds
	TaskEventUpdateDeadline int // Deadline in seconds for the scheduler to receive task update events
}

type WorkerConfig struct {
	Name             string   // Worker name used to identify the worker
	PoolSize         int      // Worker executor pool size
	Generation       int      // Worker generation
	ReportInterval   int      // Interval in seconds that the worker reports events to scheduler
	HandlerWhiteList []string // Handler white list, when specified only listed handlers are allowed to be registered. Default to '*' which means all handlers can be registered
}

type TransportConfig struct {
	Identifier string      // Scheduler id or worker name
	Type       string      // Transport type, e.g. kafka
	Role       string      // Transport role, available values are enum.TransportRole
	Kafka      KafkaConfig // Kafka config
	Grpc       GrpcConfig  // GRPC transport config
}

type ServerConfig struct {
	HttpAddress string // HTTP server address
	GrpcAddress string // GRPC server address
}

type KafkaConfig struct {
	Brokers      []string    // Broker addresses
	Topics       KafkaTopics // Topics
	GroupId      string      // Consumer group id
	MessageTTL   int         // Message TTL in seconds
	EnableSASL   bool        // Enable SASL
	SASLUsername string      // SASL username
	SASLPassword string      // SASL password
}

type KafkaTopics struct {
	Tasks    []string // Tasks topics
	Messages []string // Messages topics
}

type GrpcConfig struct {
	// Common config (applies to both Scheduler and Worker)
	APIKey                    string // Shared API key auth; Worker sends via header; Scheduler validates when Auth.Type is empty
	TLSEnable                 bool   // Enable TLS for both client and server sides
	TLSCAFile                 string // CA file; Worker: RootCAs for server verify; Scheduler: ClientCAs for mTLS client verification
	TLSCertFile               string // Certificate file; Scheduler: server cert; Worker: client cert for mTLS
	TLSKeyFile                string // Private key file; Scheduler: server key; Worker: client key for mTLS
	InsecureSkipVerify        bool   // Skip TLS verification (development only)
	SendRetryMax              int    // Max retry attempts for send (used by both Scheduler→Worker and Worker→Scheduler)
	SendRetryInitialBackoffMs int    // Initial backoff in ms
	SendRetryMaxBackoffMs     int    // Max backoff in ms
	SendRetryJitterPct        int    // Jitter percentage

	// Scheduler config
	ListenAddress string     // gRPC server listen address for Scheduler (e.g. ":8443")
	Auth          AuthConfig // Scheduler auth provider; when Type=="simple" uses APIKeys and ignores APIKey

	// Worker config
	Mode                 string   // Address discovery mode: static|dns|k8s (Worker only)
	SchedulerEndpoints   []string // Used when Mode=="static"; array of scheduler endpoints (host:port)
	DNSName              string   // Used when Mode=="dns"; resolves A records for scheduler cluster
	K8SNamespace         string   // Used when Mode=="k8s"; scheduler service namespace
	K8SService           string   // Used when Mode=="k8s"; scheduler service name
	Port                 int      // Port for DNS/K8s discovery endpoints; default 443 if <=0
	ReconnectOnSendError bool     // Worker: whether to reconnect stream on send error
	HeartbeatInterval    int      // Worker: heartbeat interval in seconds to report handlers
}

type AuthConfig struct {
	Type    string   // simple|jwt|hmac
	APIKeys []string // allowed api keys for simple
}

type SnapshotConfig struct {
	Enabled      bool          // When enabled, schedulers save snapshot files to specified S3 bucket
	MaxVersions  int           // The maximum number of snapshot versions being kept
	Interval     time.Duration // Interval to take snapshots
	Endpoint     string        // S3 endpoint
	Region       string        // S3 region
	Bucket       string        // S3 bucket
	AccessKey    string        // S3 access key
	AccessSecret string        // S3 access secret
	Secure       bool          // Whether LTS is enabled
}
