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
	Mode                      string     // Address discovery mode: static|dns|k8s
	SchedulerEndpoints        []string   // Worker-side scheduler endpoints
	ListenAddress             string     // Transport server listen address
	APIKey                    string     // API key for auth
	DNSName                   string     // DNS name for discovery
	K8SNamespace              string     // K8s namespace for service discovery
	K8SService                string     // K8s service name for discovery
	TLSEnable                 bool       // Enable TLS
	TLSCAFile                 string     // CA file path
	TLSCertFile               string     // Client/server cert file
	TLSKeyFile                string     // Client/server key file
	InsecureSkipVerify        bool       // Skip TLS verification
	SendRetryMax              int        // Max retry attempts
	SendRetryInitialBackoffMs int        // Initial backoff in ms
	SendRetryMaxBackoffMs     int        // Max backoff in ms
	SendRetryJitterPct        int        // Jitter percentage
	ReconnectOnSendError      bool       // Reconnect on send error
	HeartbeatInterval         int        // Heartbeat interval in seconds
	Auth                      AuthConfig // Auth provider config
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
