package transport

import (
	"context"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/satori/uuid"
	"math/rand"
	"os"
	"strings"
	"time"
)

var (
	// DefaultMessageTTL and other options, can be modified accordingly
	DefaultMessageTTL        = 10 // message TTL default to 10 seconds
	DefaultNumPartitions     = 3  // topic partitions default to 3
	DefaultReplicationFactor = 1  // topic replication factor default to 1
)

const HeaderFrom = "From"

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
	// Validate config
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	// Initialize topics
	topics := cfg.Kafka.Topics.Messages
	if cfg.Role == string(enum.TransportRoleWorker) {
		topics = cfg.Kafka.Topics.Tasks
	}
	createTopics(cfg.Kafka.Brokers, topics)

	// Create producer and consumer
	p, err := kafka.NewProducer(newProducerConfig(cfg.Kafka))
	if err != nil {
		return nil, fmt.Errorf("error intializing kafka producer: %v", err)
	}
	c, err := kafka.NewConsumer(newConsumerConfig(cfg.Kafka))
	if err != nil {
		return nil, fmt.Errorf("error intializing kafka consumer: %v", err)
	}
	if err = c.SubscribeTopics(topics, nil); err != nil {
		return nil, fmt.Errorf("error subscribing topics %v: %v", topics, err)
	}
	log.Info().Strs("topics", topics).Str("role", cfg.Role).
		Str("groupId", cfg.Kafka.GroupId).Msg("added subscription to topics")

	lg := zerolog.New(os.Stdout).With().Timestamp().Str("tran", "kafka").Logger()
	t := &KafkaTransport{
		c:              c,
		p:              p,
		lg:             &lg,
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

func (t *KafkaTransport) Start() error {
	go t.handleProducerEvents()
	go t.consume()
	return nil
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
		Str("sample", sampling(msg)).Msg("sending message")
	return t.p.Produce(kmsg, nil)
}

func (t *KafkaTransport) CloseSend() error {
	t.closeSend = true
	return nil
}

func (t *KafkaTransport) CloseReceive() error {
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
			if !isTimeout(err) {
				t.lg.Err(err).Msg("error reading message from brokers")
			}
			continue
		}

		if int(time.Now().Sub(msg.Timestamp).Seconds()) > t.ttl {
			t.lg.Info().Msgf("ignoring tasks dispatched %v seconds ago", t.ttl)
			continue
		}

		t.lg.Trace().Str("sample", sampling(msg.Value)).Msgf("handling message")
		if res, err = t.omr(from(msg.Headers), string(msg.Key), msg.Value); err != nil {
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

// createTopics creates specified topics silently
func createTopics(brokers, topics []string) {
	cfg := &kafka.ConfigMap{
		"bootstrap.servers": strings.Join(brokers, ","),
	}
	client, err := kafka.NewAdminClient(cfg)
	if err != nil {
		log.Err(err).Msg("failed to create kafka admin client")
		return
	}

	opt := kafka.SetAdminOperationTimeout(time.Second * 60)
	res, err := client.CreateTopics(context.Background(), topicSpecs(topics), opt)
	if err != nil {
		log.Err(err).Msg("failed to create kafka topics")
		return
	}
	for _, r := range res {
		if r.Error.Code() != kafka.ErrNoError {
			log.Err(r.Error).Str("topic", r.Topic).Msg("error creating specified topic")
		}
	}
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

func topicSpecs(topics []string) []kafka.TopicSpecification {
	specs := make([]kafka.TopicSpecification, 0)
	for i, _ := range topics {
		specs = append(specs, kafka.TopicSpecification{
			Topic:             topics[i],
			NumPartitions:     DefaultNumPartitions,
			ReplicationFactor: DefaultReplicationFactor,
		})
	}
	return specs
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

// Kafka Timeout is not considered an error because it is raised by
// ReadMessage in absence of messages.
func isTimeout(err error) bool {
	kerr, ok := err.(kafka.Error)
	if !ok {
		return false
	}
	return kerr.Code() == kafka.ErrTimedOut || kerr.Code() == kafka.ErrTimedOutQueue
}

func sampling(msg []byte) string {
	tmp := string(msg)
	l := len(tmp)
	if l > 100 {
		l = 100
	}
	return tmp[:l]
}
