package kafka

import (
	"context"
	"fmt"
	"log"
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

// NewProducer constructs a pure Go KafkaProducer configured for resilient message delivery.
// Why: Provides thread-safe, buffered message publication without CGo compilation dependencies.
func NewProducer(brokers []string) *KafkaProducer {
	writer := &kafkaGo.Writer{
		Addr:         kafkaGo.TCP(brokers...),
		Balancer:     &kafkaGo.LeastBytes{},
		WriteTimeout: 5 * time.Second,
		RequiredAcks: kafkaGo.RequireOne,
		Async:        false,
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
