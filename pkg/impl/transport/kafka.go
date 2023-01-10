package transport

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/satori/uuid"
	"math/rand"
	"strings"
	"time"
)

var (
	DefaultMessageTTL = 10 // 10 seconds
	HeaderFrom        = "From"
)

type KafkaTransport struct {
	c              *kafka.Consumer
	p              *kafka.Producer
	lg             *zerolog.Logger
	cfg            *config.TransportConfig
	omr            types.OnMessageReceived
	ttl            int
	closeSend      bool
	closeReceiving bool
}

func NewKafkaTransport(cfg *config.TransportConfig) (*KafkaTransport, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	p, err := kafka.NewProducer(newProducerConfig(cfg.Kafka))
	if err != nil {
		return nil, fmt.Errorf("error intializing kafka producer: %v", err)
	}
	c, err := kafka.NewConsumer(newConsumerConfig(cfg.Kafka))
	if err != nil {
		return nil, fmt.Errorf("error intializing kafka consumer: %v", err)
	}
	topics := cfg.Kafka.Topics.Messages
	if cfg.Role == string(enum.TransportRoleWorker) {
		topics = cfg.Kafka.Topics.Tasks
	}
	if err = c.SubscribeTopics(topics, nil); err != nil {
		return nil, fmt.Errorf("error subscribing topics %v: %v", topics, err)
	}
	log.Info().Strs("topics", topics).Str("role", cfg.Role).
		Str("groupId", cfg.Kafka.GroupId).Msg("added subscription to topics")

	t := &KafkaTransport{
		c:              c,
		p:              p,
		lg:             new(zerolog.Logger),
		cfg:            cfg,
		closeReceiving: false,
		closeSend:      false,
	}

	t.ttl = DefaultMessageTTL
	if cfg.Kafka.MessageTTL > 0 {
		t.ttl = cfg.Kafka.MessageTTL
	}

	return t, nil
}

func (t *KafkaTransport) OnReceive(omr types.OnMessageReceived) {
	t.omr = omr
}

func (t *KafkaTransport) Send(from, to string, msg []byte) error {
	topic := t.selectTopic()
	var key []byte
	if len(to) != 0 {
		key = []byte(to)
	}
	kmsg := &kafka.Message{
		Key: key,
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Headers: []kafka.Header{
			{
				Key:   HeaderFrom,
				Value: []byte(from),
			},
		},
		Value:         msg,
		Timestamp:     time.Now(),
		TimestampType: kafka.TimestampCreateTime,
	}
	log.Trace().Str("from", from).Str("to", to).Str("topic", topic).
		Bytes("value", msg).Msg("sending message")
	return t.p.Produce(kmsg, nil)
}

func (t *KafkaTransport) Start() error {
	go t.handleProducerEvents()
	go t.consume()
	return nil
}

func (t *KafkaTransport) CloseSend() error {
	t.closeSend = true
	return nil
}

func (t *KafkaTransport) CloseReceiving() error {
	t.closeReceiving = true
	return nil
}

func (t *KafkaTransport) consume() {
	var (
		err error
		res []byte
		msg *kafka.Message
	)

	defer func() {
		if r := recover(); r != nil {
			t.lg.Error().Msgf("kafka transport consumer panicked: %v", r)
		}
	}()

	for {
		if t.closeReceiving {
			return
		}
		if msg, err = t.c.ReadMessage(time.Second); err != nil {
			t.lg.Err(err).Msg("error reading message from brokers")
			continue
		}

		if int(time.Now().Sub(msg.Timestamp).Seconds()) > t.ttl {
			t.lg.Info().Msgf("ignoring tasks dispatched %v seconds ago", t.ttl)
			continue
		}

		t.lg.Trace().Bytes("value", msg.Value).Msgf("handling message")
		if res, err = t.omr(from(msg.Headers), msg.Value); err != nil {
			t.lg.Err(err).Bytes("result", res).Msg("error handling message")
		}
	}
}

func (t *KafkaTransport) handleProducerEvents() {
	for e := range t.p.Events() {
		if t.closeSend {
			return
		}

		switch ev := e.(type) {
		case kafka.Error:
			t.lg.Err(ev).Msg("producer error")
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				t.lg.Err(ev.TopicPartition.Error).Msg("failed to enqueue message in kafka")
			} else {
				t.lg.Trace().Str("topic", *ev.TopicPartition.Topic).
					Int32("partition", ev.TopicPartition.Partition).
					Int64("offset", int64(ev.TopicPartition.Offset)).Msg("enqueued message")
			}
		}
	}
}

func (t *KafkaTransport) selectTopic() string {
	topics := t.cfg.Kafka.Topics.Tasks
	if t.cfg.Role == string(enum.TransportRoleWorker) {
		topics = t.cfg.Kafka.Topics.Messages
	}
	if len(topics) == 0 {
		return ""
	}
	return topics[rand.Intn(len(topics))]
}

func newConsumerConfig(conf config.KafkaConfig) *kafka.ConfigMap {
	return &kafka.ConfigMap{
		"bootstrap.servers":         strings.Join(conf.Brokers, ","),
		"group.id":                  conf.GroupId,
		"client.id":                 uuid.NewV4().String(),
		"api.version.request":       "true",
		"auto.offset.reset":         "latest", // Must be latest
		"enable.auto.commit":        "true",
		"heartbeat.interval.ms":     3 * enum.Second,    // 3s
		"session.timeout.ms":        30 * enum.Second,   // 30s
		"max.poll.interval.ms":      7200 * enum.Second, // 7200s, 2hours
		"message.max.bytes":         500 * enum.KB,      // 100KB
		"fetch.max.bytes":           500 * enum.KB,      // 100KB
		"max.partition.fetch.bytes": 500 * enum.KB,      // 100KB
		"security.protocol":         "PLAINTEXT",

		// "group.instance.id": ""
		// TODO: Enable static group membership.
		// Static group members are able to leave and rejoin a group within the configured session.timeout.ms without prompting a group rebalance.
		// This should be used in combination with a larger session.timeout.ms to avoid group rebalances caused by transient unavailability (e.g. process restarts).
		// Requires broker version >= 2.3.0.
	}
}

func newProducerConfig(conf config.KafkaConfig) *kafka.ConfigMap {
	return &kafka.ConfigMap{
		"acks":                "all", // All replicas ack
		"bootstrap.servers":   strings.Join(conf.Brokers, ","),
		"client.id":           uuid.NewV4().String(),
		"api.version.request": "true",
		"security.protocol":   "PLAINTEXT",
		"retries":             3,
		"retry.backoff.ms":    100,
		"linger.ms":           200,
	}
}

func from(headers []kafka.Header) string {
	for _, v := range headers {
		if v.Key == HeaderFrom {
			return string(v.Value)
		}
	}
	return ""
}

func validateConfig(cfg *config.TransportConfig) error {
	if cfg.Role != string(enum.TransportRoleScheduler) && cfg.Role != string(enum.TransportRoleWorker) {
		return fmt.Errorf("TransportConfig.Role was not specified")
	}
	if len(cfg.Kafka.Brokers) == 0 {
		return fmt.Errorf("TransportConfig.Kafka.Brokers was not specified")
	}
	if cfg.Kafka.GroupId == "" {
		return fmt.Errorf("TransportConfig.Kafka.GroupId was not specified")
	}
	if len(cfg.Kafka.Topics.Tasks) == 0 || len(cfg.Kafka.Topics.Messages) == 0 {
		return fmt.Errorf("TransportConfig.Kafka.Topics was not specified or missing value")
	}
	return nil
}
