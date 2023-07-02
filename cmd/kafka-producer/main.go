package main

import (
	"encoding/json"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog/log"
	"github.com/satori/uuid"
	"os"
	"strings"
	"time"
)

var task = types.Task{
	Handler:      "test",
	TenantId:     "1",
	Uid:          "test-task-1",
	SchedulerId:  "scheduler-1",
	Config:       `{"key": "value"}`,
	RestartTimes: 0,
	LastRun:      nil,
}

var brokers = strings.Split(os.Getenv("KEEL_TRANSPORT_KAFKA_BROKERS"), ",")

var topic = "cloud-keel-tasks-test-0"

func main() {
	p, err := kafka.NewProducer(producerConfig(brokers))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create kafka producer")
	}
	go handleProducerEvents(p)
	if err = send(p, topic, task); err != nil {
		log.Fatal().Err(err).Msg("error sending new task")
	}
	time.Sleep(time.Second * 10)
}

func send(p *kafka.Producer, topic string, v types.Task) error {
	byt, err := json.Marshal(v)
	if err != nil {
		log.Err(err).Msg("failed to marshal message")
		return err
	}

	msg := &kafka.Message{
		// Key: []byte(v.SchedulerId),
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Value:         byt,
		Timestamp:     time.Now(),
		TimestampType: kafka.TimestampCreateTime,
	}
	if err = p.Produce(msg, nil); err != nil {
		log.Err(err).Msg("failed to enqueue message")
	}
	return err
}

func producerConfig(brokers []string) *kafka.ConfigMap {
	return &kafka.ConfigMap{
		"acks":                "all", // All replicas ack
		"bootstrap.servers":   strings.Join(brokers, ","),
		"client.id":           uuid.NewV4(),
		"api.version.request": "true",
		"security.protocol":   "PLAINTEXT",
		"retries":             3,
		"retry.backoff.ms":    100,
		"linger.ms":           200,
	}
}

func handleProducerEvents(p *kafka.Producer) {
	for e := range p.Events() {
		switch ev := e.(type) {
		case kafka.Error:
			log.Err(ev).Msg("producer error")
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				log.Err(ev.TopicPartition.Error).Msg("failed to enqueue message in kafka")
			} else {
				log.Trace().Str("topic", *ev.TopicPartition.Topic).
					Int32("partition", ev.TopicPartition.Partition).
					Int64("offset", int64(ev.TopicPartition.Offset)).Msg("enqueued message")
			}
		}
	}
}
