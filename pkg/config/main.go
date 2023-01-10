package config

import (
	"github.com/rs/zerolog"
	"time"
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
	Id                      string // Scheduler id, also used to identify partition
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
	Name           string // Worker name used to identify the worker
	PoolSize       int    // Worker executor pool size
	Generation     int    // Worker generation
	ReportInterval int    // Interval in seconds that the worker reports events to scheduler
}

type TransportConfig struct {
	Type  string      // Transport type, e.g. kafka
	Role  string      // Transport role, available values are enum.TransportRole
	Kafka KafkaConfig // Kafka config
}

type ServerConfig struct {
	HttpAddress string // HTTP server address
	GrpcAddress string // GRPC server address
}

type KafkaConfig struct {
	Brokers    []string    // Broker addresses
	Topics     KafkaTopics // Topics
	GroupId    string      // Consumer group id
	MessageTTL int         // Message TTL in seconds
}

type KafkaTopics struct {
	Tasks    []string // Tasks topics
	Messages []string // Messages topics
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
