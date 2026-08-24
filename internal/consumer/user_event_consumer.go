package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	kafkaGo "github.com/segmentio/kafka-go"

	"github.com/Sall-lah/store_order/internal/service"
)

// MessageReader defines the interface for consuming messages from Kafka partitions.
// Why: Decouples the consumer business logic from the concrete Kafka reader to facilitate unit testing with mocks.
type MessageReader interface {
	FetchMessage(ctx context.Context) (kafkaGo.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkaGo.Message) error
	Close() error
}

// UserEventEnvelope models platform-wide asynchronous event structures.
type UserEventEnvelope struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Event     string          `json:"event"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Producer  string          `json:"producer"`
	Data      json.RawMessage `json:"data"`

	// Flat fallbacks when data is not wrapped in a nested object
	UserID string `json:"user_id"`
	UserId string `json:"userId"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// UserDeletedPayload represents the data payload for user.deleted events.
type UserDeletedPayload struct {
	UserID string `json:"user_id"`
	UserId string `json:"userId"`
	ID     string `json:"id"`
	Email  string `json:"email"`
}

// UserEventConsumer listens for user lifecycle events from Kafka (specifically user.deleted)
// and triggers order cancellation and PII anonymization in store_order.
type UserEventConsumer struct {
	reader       MessageReader
	orderService service.OrderService
	stopChan     chan struct{}
	wg           sync.WaitGroup
	isDevMode    bool
}

// newSmartDialer creates a Kafka dialer that handles container DNS and local development fallbacks.
// Why: Resolves advertised broker container hostnames during local testing without breaking intra-container networking.
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

// NewUserEventConsumer constructs a UserEventConsumer connected to Kafka brokers with a consumer group.
// Why: Encapsulates partition assignment and offset tracking for horizontal scaling across microservice instances.
func NewUserEventConsumer(
	brokers []string,
	topic string,
	groupID string,
	orderService service.OrderService,
	isDevMode bool,
) *UserEventConsumer {
	if topic == "" {
		topic = "user.events"
	}
	if groupID == "" {
		groupID = "store_order_user_events"
	}

	dialer := newSmartDialer()
	reader := kafkaGo.NewReader(kafkaGo.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		Dialer:         dialer,
		CommitInterval: time.Second,
		StartOffset:    kafkaGo.FirstOffset,
	})

	return NewUserEventConsumerWithReader(reader, orderService, isDevMode)
}

// NewUserEventConsumerWithReader constructs a consumer instance with an injected reader.
// Why: Allows unit tests to provide mock message readers without requiring a running Kafka cluster.
func NewUserEventConsumerWithReader(
	reader MessageReader,
	orderService service.OrderService,
	isDevMode bool,
) *UserEventConsumer {
	return &UserEventConsumer{
		reader:       reader,
		orderService: orderService,
		stopChan:     make(chan struct{}),
		isDevMode:    isDevMode,
	}
}

// UserEventData represents extracted domain payload from user lifecycle Kafka events.
type UserEventData struct {
	UserID    string
	EventType string
	Reason    string
}

// ExtractUserLifecycleEvent inspects the message payload and returns user ID, event type, and optional reason.
// Why: Robustly handles varied envelope serialization styles (nested data vs flat fields) while filtering supported user lifecycle events.
func ExtractUserLifecycleEvent(payload []byte) (*UserEventData, error) {
	if len(payload) == 0 {
		return nil, errors.New("empty message payload")
	}

	var env UserEventEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("invalid json payload: %w", err)
	}

	eventType := strings.ToLower(strings.TrimSpace(env.EventType))
	if eventType == "" {
		eventType = strings.ToLower(strings.TrimSpace(env.Event))
	}
	if eventType == "" {
		eventType = strings.ToLower(strings.TrimSpace(env.Type))
	}

	// Supported events: user.deleted, user.banned
	if eventType != "user.deleted" && eventType != "user.banned" {
		if eventType != "" {
			return nil, nil // Ignored event type (e.g. user.created, user.updated)
		}
	}

	var targetUserID string
	var reason string

	// Try extracting from nested Data
	if len(env.Data) > 0 && string(env.Data) != "null" {
		var data struct {
			UserID string `json:"user_id"`
			UserId string `json:"userId"`
			ID     string `json:"id"`
			Email  string `json:"email"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(env.Data, &data); err == nil {
			if data.UserID != "" {
				targetUserID = data.UserID
			} else if data.UserId != "" {
				targetUserID = data.UserId
			} else if data.ID != "" {
				targetUserID = data.ID
			}
			if data.Reason != "" {
				reason = data.Reason
			}
		}
	}

	// Fallback to top-level fields
	if targetUserID == "" {
		if env.UserID != "" {
			targetUserID = env.UserID
		} else if env.UserId != "" {
			targetUserID = env.UserId
		} else if env.ID != "" {
			targetUserID = env.ID
		}
	}
	if reason == "" && env.Reason != "" {
		reason = env.Reason
	}

	targetUserID = strings.TrimSpace(targetUserID)
	if targetUserID == "" {
		return nil, errors.New("user ID missing from user event payload")
	}

	if eventType == "" {
		eventType = "user.deleted"
	}

	return &UserEventData{
		UserID:    targetUserID,
		EventType: eventType,
		Reason:    strings.TrimSpace(reason),
	}, nil
}

// ExtractUserDeletedID inspects the message payload and returns the target user ID if the event is a user.deleted event.
// Why: Retains compatibility for callers expecting a user.deleted specific parser.
func ExtractUserDeletedID(payload []byte) (string, bool, error) {
	event, err := ExtractUserLifecycleEvent(payload)
	if err != nil {
		return "", false, err
	}
	if event == nil || event.EventType != "user.deleted" {
		return "", false, nil
	}
	return event.UserID, true, nil
}

// ProcessMessage parses a Kafka message and executes the appropriate user lifecycle handler (user.deleted or user.banned).
// Why: Provides a testable, decoupled message handling unit that routes lifecycle events to domain handlers and returns whether the message should be committed.
func (c *UserEventConsumer) ProcessMessage(ctx context.Context, msg kafkaGo.Message) error {
	event, err := ExtractUserLifecycleEvent(msg.Value)
	if err != nil {
		log.Printf("[UserEventConsumer] Skipping invalid event on partition %d offset %d: %v", msg.Partition, msg.Offset, err)
		return nil // Commit offset to prevent poison pill blocking
	}

	if event == nil {
		return nil
	}

	switch event.EventType {
	case "user.deleted":
		log.Printf("[UserEventConsumer] Processing user.deleted event for userID: %s (topic: %s, partition: %d, offset: %d)",
			event.UserID, msg.Topic, msg.Partition, msg.Offset)

		if err := c.orderService.HandleUserDeleted(ctx, event.UserID); err != nil {
			log.Printf("[UserEventConsumer] Error executing HandleUserDeleted for userID %s: %v", event.UserID, err)
			return err
		}

		log.Printf("[UserEventConsumer] Successfully processed user.deleted event for userID: %s", event.UserID)

	case "user.banned":
		log.Printf("[UserEventConsumer] Processing user.banned event for userID: %s (reason: %s, topic: %s, partition: %d, offset: %d)",
			event.UserID, event.Reason, msg.Topic, msg.Partition, msg.Offset)

		if err := c.orderService.HandleUserBanned(ctx, event.UserID, event.Reason); err != nil {
			log.Printf("[UserEventConsumer] Error executing HandleUserBanned for userID %s: %v", event.UserID, err)
			return err
		}

		log.Printf("[UserEventConsumer] Successfully processed user.banned event for userID: %s", event.UserID)
	}

	return nil
}

// Start initiates the asynchronous event consumption loop in a dedicated goroutine.
// Why: Runs concurrent message processing alongside HTTP/gRPC servers without blocking application startup.
func (c *UserEventConsumer) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.run(ctx)
	log.Println("[UserEventConsumer] Background user event consumer started.")
}

// Stop signals the consumer loop to terminate and waits for in-flight message processing to finish.
// Why: Ensures clean partition rebalance and finishes active database transactions during graceful shutdown.
func (c *UserEventConsumer) Stop() {
	select {
	case <-c.stopChan:
		// already closed
	default:
		close(c.stopChan)
	}
	c.wg.Wait()
	log.Println("[UserEventConsumer] Background user event consumer stopped gracefully.")
}

// Close terminates the underlying Kafka reader connection and stops the consumer loop.
// Why: Releases network connections and broker partition locks upon service termination.
func (c *UserEventConsumer) Close() error {
	c.Stop()
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}

func (c *UserEventConsumer) run(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
					return
				}
				select {
				case <-c.stopChan:
					return
				case <-ctx.Done():
					return
				default:
					log.Printf("[UserEventConsumer] Error fetching message: %v", err)
					time.Sleep(300 * time.Millisecond)
					continue
				}
			}

			if err := c.ProcessMessage(ctx, msg); err != nil {
				log.Printf("[UserEventConsumer] Error processing message at offset %d: %v", msg.Offset, err)
				time.Sleep(300 * time.Millisecond)
				continue
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[UserEventConsumer] Error committing offset %d: %v", msg.Offset, err)
			}
		}
	}
}
