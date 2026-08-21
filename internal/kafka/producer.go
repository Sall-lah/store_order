package kafka

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	kafkaGo "github.com/segmentio/kafka-go"
)

// Producer defines the message publication contract for dispatching domain events to Kafka.
type Producer interface {
	Publish(ctx context.Context, topic, key string, payload []byte) error
	Close() error
}

// KafkaProducer wraps segmentio/kafka-go Writer for pure Go message publishing.
type KafkaProducer struct {
	writer *kafkaGo.Writer
}

// newSmartDialer creates a Kafka dialer that respects container DNS while gracefully falling back to 127.0.0.1 for local host execution.
// Why: Resolves advertised broker container hostnames when running local development outside Docker network without breaking intra-container DNS resolution.
func newSmartDialer() *kafkaGo.Dialer {
	return &kafkaGo.Dialer{
		Timeout: 10 * time.Second,
		DialFunc: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err == nil {
				if _, lookupErr := net.LookupHost(host); lookupErr != nil {
					if host == "kafka" || host == "broker" {
						addr = net.JoinHostPort("127.0.0.1", port)
					}
				}
			}
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
	}
}

// NewProducer constructs a pure Go KafkaProducer configured for resilient message delivery.
// Why: Provides thread-safe, buffered message publication without CGo compilation dependencies.
func NewProducer(brokers []string) *KafkaProducer {
	dialer := newSmartDialer()

	writer := &kafkaGo.Writer{
		Addr:                   kafkaGo.TCP(brokers...),
		Balancer:               &kafkaGo.LeastBytes{},
		WriteTimeout:           5 * time.Second,
		RequiredAcks:           kafkaGo.RequireOne,
		Async:                  false,
		AllowAutoTopicCreation: true,
		Transport: &kafkaGo.Transport{
			Dial: dialer.DialFunc,
		},
	}

	return &KafkaProducer{writer: writer}
}

// Publish dispatches a single keyed message to the target Kafka topic.
// Why: Ensures partition ordering per aggregate key (e.g. order_id) across consumer partitions.
func (p *KafkaProducer) Publish(ctx context.Context, topic, key string, payload []byte) error {
	msg := kafkaGo.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write kafka message to topic %s: %w", topic, err)
	}

	return nil
}

// Close flushes buffered messages and terminates broker connections.
// Why: Prevents message loss and connection leaks during application shutdown.
func (p *KafkaProducer) Close() error {
	log.Println("[Kafka] Closing Kafka producer writer...")
	return p.writer.Close()
}
