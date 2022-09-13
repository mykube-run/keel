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
	Id                string // Scheduler id, also used to identify partition
	Zone              string
	Port              int
	Numbers           int // Number of schedulers within the same zone
	Address           string
	AdvertisedAddress string
	ScheduleInterval  int // Schedule interval in seconds
	StaleCheckDelay   int // Stale tasks check delay after start up in seconds
}

type WorkerConfig struct {
	Name           string
	PoolSize       int
	Generation     int
	ReportInterval int
}

type TransportConfig struct {
	Type  string // Transport type, e.g. kafka
	Role  string // Transport role, available values are enum.TransportRole
	Kafka KafkaConfig
}

type ServerConfig struct {
	HttpAddress string //httpServer address
	GrpcAddress string //grpcServer address
}

type KafkaConfig struct {
	Brokers    []string    // Broker addresses
	Topics     KafkaTopics // Topics
	GroupId    string      // Consumer group id
	MessageTTL int         // Message TTL in seconds
}

type KafkaTopics struct {
	Tasks    []string
	Messages []string
}

type SnapshotConfig struct {
	Enabled      bool
	MaxVersions  int
	Interval     time.Duration
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	AccessSecret string
	Secure       bool
}
