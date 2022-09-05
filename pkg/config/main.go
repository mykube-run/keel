package config

import (
	"github.com/rs/zerolog"
	"time"
)

type Config struct {
	Log       LogConfig
	Database  DatabaseConfig
	Scheduler SchedulerConfig
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
	Id                string // id also used to identify partition
	Numbers           int
	Zone              string
	Port              int
	Address           string
	AdvertisedAddress string
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
