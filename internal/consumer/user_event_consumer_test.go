package consumer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	kafkaGo "github.com/segmentio/kafka-go"

	"github.com/Sall-lah/store_order/internal/consumer"
	"github.com/Sall-lah/store_order/internal/integration/midtrans"
	"github.com/Sall-lah/store_order/internal/repository"
	"github.com/Sall-lah/store_order/internal/service"
)

// mockOrderService implements service.OrderService for testing consumer behaviors.
type mockOrderService struct {
	deletedUsers []string
	bannedUsers  []string
	banReasons   []string
	shouldError  bool
	mu           sync.Mutex
}

func newMockOrderService() *mockOrderService {
	return &mockOrderService{
		deletedUsers: make([]string, 0),
		bannedUsers:  make([]string, 0),
		banReasons:   make([]string, 0),
	}
}

func (m *mockOrderService) HandleUserDeleted(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shouldError {
		return errors.New("service failure")
	}
	m.deletedUsers = append(m.deletedUsers, userID)
	return nil
}

func (m *mockOrderService) HandleUserBanned(ctx context.Context, userID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shouldError {
		return errors.New("service failure")
	}
	m.bannedUsers = append(m.bannedUsers, userID)
	m.banReasons = append(m.banReasons, reason)
	return nil
}

func (m *mockOrderService) Checkout(ctx context.Context, userID, userEmail string, req service.CheckoutRequest) (*service.OrderResponse, error) {
	return nil, nil
}
func (m *mockOrderService) GetCustomerOrder(ctx context.Context, userID, orderID string) (*service.OrderResponse, error) {
	return nil, nil
}
func (m *mockOrderService) ListCustomerOrders(ctx context.Context, userID string, limit, offset int) (*service.OrderListResponse, error) {
	return nil, nil
}
func (m *mockOrderService) CancelCustomerOrder(ctx context.Context, userID, orderID string) (*service.OrderResponse, error) {
	return nil, nil
}
func (m *mockOrderService) ProcessMidtransWebhook(ctx context.Context, notif midtrans.WebhookNotification) error {
	return nil
}
func (m *mockOrderService) AdminListOrders(ctx context.Context, filter repository.OrderFilter) (*service.OrderListResponse, error) {
	return nil, nil
}
func (m *mockOrderService) AdminUpdateStatus(ctx context.Context, orderID string, newStatus string, courierName string, receiptNumber string) (*service.OrderResponse, error) {
	return nil, nil
}
func (m *mockOrderService) SimulatePaymentSuccess(ctx context.Context, orderID string) (*service.OrderResponse, error) {
	return nil, nil
}
func (m *mockOrderService) SimulateOrderCancel(ctx context.Context, orderID string) (*service.OrderResponse, error) {
	return nil, nil
}
func (m *mockOrderService) SimulateOrderExpire(ctx context.Context, orderID string) (*service.OrderResponse, error) {
	return nil, nil
}

// mockMessageReader implements consumer.MessageReader for testing the consumer loop.
type mockMessageReader struct {
	messages  []kafkaGo.Message
	committed []kafkaGo.Message
	index     int
	closed    bool
	mu        sync.Mutex
}

func (r *mockMessageReader) FetchMessage(ctx context.Context) (kafkaGo.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return kafkaGo.Message{}, errors.New("reader closed")
	}
	if r.index >= len(r.messages) {
		time.Sleep(50 * time.Millisecond)
		return kafkaGo.Message{}, context.Canceled
	}
	msg := r.messages[r.index]
	r.index++
	return msg, nil
}

func (r *mockMessageReader) CommitMessages(ctx context.Context, msgs ...kafkaGo.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.committed = append(r.committed, msgs...)
	return nil
}

func (r *mockMessageReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

// TestExtractUserDeletedID tests parsing variations and filtering for user.deleted events.
// Why: Guarantees compatibility across different producer payload formats while discarding irrelevant domain events.
func TestExtractUserDeletedID(t *testing.T) {
	cases := []struct {
		name          string
		payload       string
		expectedID    string
		expectedFound bool
		expectErr     bool
	}{
		{
			name:          "nested data with snake_case user_id",
			payload:       `{"event_id":"evt_1","event_type":"user.deleted","timestamp":"2026-08-24T00:00:00Z","producer":"store_user","data":{"user_id":"usr_1001","email":"u1@example.com"}}`,
			expectedID:    "usr_1001",
			expectedFound: true,
			expectErr:     false,
		},
		{
			name:          "nested data with camelCase userId",
			payload:       `{"event_id":"evt_2","event_type":"user.deleted","data":{"userId":"usr_1002"}}`,
			expectedID:    "usr_1002",
			expectedFound: true,
			expectErr:     false,
		},
		{
			name:          "flat payload with user_id",
			payload:       `{"event_type":"user.deleted","user_id":"usr_1003"}`,
			expectedID:    "usr_1003",
			expectedFound: true,
			expectErr:     false,
		},
		{
			name:          "type alias with id field",
			payload:       `{"type":"user.deleted","data":{"id":"usr_1004"}}`,
			expectedID:    "usr_1004",
			expectedFound: true,
			expectErr:     false,
		},
		{
			name:          "other event type ignored",
			payload:       `{"event_type":"user.created","data":{"user_id":"usr_ignore"}}`,
			expectedID:    "",
			expectedFound: false,
			expectErr:     false,
		},
		{
			name:          "invalid json payload errors",
			payload:       `{malformed_json`,
			expectedID:    "",
			expectedFound: false,
			expectErr:     true,
		},
		{
			name:          "missing user ID in user.deleted errors",
			payload:       `{"event_type":"user.deleted","data":{}}`,
			expectedID:    "",
			expectedFound: false,
			expectErr:     true,
		},
		{
			name:          "empty payload errors",
			payload:       "",
			expectedID:    "",
			expectedFound: false,
			expectErr:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, found, err := consumer.ExtractUserDeletedID([]byte(tc.payload))
			if tc.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if found != tc.expectedFound {
				t.Errorf("expected found %v, got %v", tc.expectedFound, found)
			}
			if id != tc.expectedID {
				t.Errorf("expected id %s, got %s", tc.expectedID, id)
			}
		})
	}
}

// TestUserEventConsumer_ProcessMessage verifies single message dispatch and error recovery.
// Why: Validates that valid events trigger order cleanup and malformed payloads are safely committed.
func TestUserEventConsumer_ProcessMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("valid user.deleted dispatches to OrderService", func(t *testing.T) {
		svc := newMockOrderService()
		c := consumer.NewUserEventConsumerWithReader(&mockMessageReader{}, svc, true)

		msg := kafkaGo.Message{
			Topic:     "user.events",
			Partition: 0,
			Offset:    10,
			Value:     []byte(`{"event_type":"user.deleted","data":{"user_id":"usr_success_1"}}`),
		}

		if err := c.ProcessMessage(ctx, msg); err != nil {
			t.Fatalf("ProcessMessage failed: %v", err)
		}

		if len(svc.deletedUsers) != 1 || svc.deletedUsers[0] != "usr_success_1" {
			t.Errorf("expected deletedUsers [usr_success_1], got %v", svc.deletedUsers)
		}
	})

	t.Run("non user.deleted event is skipped without error", func(t *testing.T) {
		svc := newMockOrderService()
		c := consumer.NewUserEventConsumerWithReader(&mockMessageReader{}, svc, true)

		msg := kafkaGo.Message{
			Topic:     "user.events",
			Partition: 0,
			Offset:    11,
			Value:     []byte(`{"event_type":"user.updated","data":{"user_id":"usr_skip"}}`),
		}

		if err := c.ProcessMessage(ctx, msg); err != nil {
			t.Fatalf("ProcessMessage failed: %v", err)
		}

		if len(svc.deletedUsers) != 0 {
			t.Errorf("expected 0 deletedUsers, got %v", svc.deletedUsers)
		}
	})

	t.Run("malformed message payload is skipped without throwing error", func(t *testing.T) {
		svc := newMockOrderService()
		c := consumer.NewUserEventConsumerWithReader(&mockMessageReader{}, svc, true)

		msg := kafkaGo.Message{
			Topic:     "user.events",
			Partition: 0,
			Offset:    12,
			Value:     []byte(`{bad_json`),
		}

		if err := c.ProcessMessage(ctx, msg); err != nil {
			t.Fatalf("ProcessMessage failed: %v", err)
		}

		if len(svc.deletedUsers) != 0 {
			t.Errorf("expected 0 deletedUsers, got %v", svc.deletedUsers)
		}
	})

	t.Run("service error bubbles up for retry", func(t *testing.T) {
		svc := newMockOrderService()
		svc.shouldError = true
		c := consumer.NewUserEventConsumerWithReader(&mockMessageReader{}, svc, true)

		msg := kafkaGo.Message{
			Topic:     "user.events",
			Partition: 0,
			Offset:    13,
			Value:     []byte(`{"event_type":"user.deleted","data":{"user_id":"usr_err"}}`),
		}

		if err := c.ProcessMessage(ctx, msg); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("valid user.banned dispatches to HandleUserBanned with reason", func(t *testing.T) {
		svc := newMockOrderService()
		c := consumer.NewUserEventConsumerWithReader(&mockMessageReader{}, svc, true)

		msg := kafkaGo.Message{
			Topic:     "user.events",
			Partition: 0,
			Offset:    14,
			Value:     []byte(`{"event_type":"user.banned","data":{"user_id":"usr_ban_dispatch_1","reason":"fraudulent chargebacks"}}`),
		}

		if err := c.ProcessMessage(ctx, msg); err != nil {
			t.Fatalf("ProcessMessage failed: %v", err)
		}

		if len(svc.bannedUsers) != 1 || svc.bannedUsers[0] != "usr_ban_dispatch_1" {
			t.Errorf("expected bannedUsers [usr_ban_dispatch_1], got %v", svc.bannedUsers)
		}
		if len(svc.banReasons) != 1 || svc.banReasons[0] != "fraudulent chargebacks" {
			t.Errorf("expected banReasons [fraudulent chargebacks], got %v", svc.banReasons)
		}
		if len(svc.deletedUsers) != 0 {
			t.Errorf("expected 0 deletedUsers, got %v", svc.deletedUsers)
		}
	})

	t.Run("service error on user.banned bubbles up for retry", func(t *testing.T) {
		svc := newMockOrderService()
		svc.shouldError = true
		c := consumer.NewUserEventConsumerWithReader(&mockMessageReader{}, svc, true)

		msg := kafkaGo.Message{
			Topic:     "user.events",
			Partition: 0,
			Offset:    15,
			Value:     []byte(`{"event_type":"user.banned","data":{"user_id":"usr_ban_err","reason":"spam"}}`),
		}

		if err := c.ProcessMessage(ctx, msg); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestExtractUserLifecycleEvent verifies parsing of user.deleted and user.banned events.
// Why: Guarantees accurate discrimination between account deletion and account banning payloads.
func TestExtractUserLifecycleEvent(t *testing.T) {
	cases := []struct {
		name          string
		payload       string
		expectedID    string
		expectedType  string
		expectedReason string
		expectNil     bool
		expectErr     bool
	}{
		{
			name:           "user.banned with nested reason",
			payload:        `{"event_type":"user.banned","data":{"user_id":"usr_ban_99","reason":"abuse"}}`,
			expectedID:     "usr_ban_99",
			expectedType:   "user.banned",
			expectedReason: "abuse",
			expectNil:      false,
			expectErr:      false,
		},
		{
			name:           "user.deleted with nested user_id",
			payload:        `{"event_type":"user.deleted","data":{"user_id":"usr_del_99"}}`,
			expectedID:     "usr_del_99",
			expectedType:   "user.deleted",
			expectedReason: "",
			expectNil:      false,
			expectErr:      false,
		},
		{
			name:      "unsupported event type user.password_changed returns nil without error",
			payload:   `{"event_type":"user.password_changed","data":{"user_id":"usr_pass"}}`,
			expectNil: true,
			expectErr: false,
		},
		{
			name:      "invalid json payload returns error",
			payload:   `{invalid_json`,
			expectNil: true,
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := consumer.ExtractUserLifecycleEvent([]byte(tc.payload))
			if tc.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expectNil {
				if data != nil {
					t.Errorf("expected nil data, got %+v", data)
				}
				return
			}
			if data == nil {
				t.Fatal("expected non-nil data, got nil")
			}
			if data.UserID != tc.expectedID {
				t.Errorf("expected userID %s, got %s", tc.expectedID, data.UserID)
			}
			if data.EventType != tc.expectedType {
				t.Errorf("expected eventType %s, got %s", tc.expectedType, data.EventType)
			}
			if data.Reason != tc.expectedReason {
				t.Errorf("expected reason %s, got %s", tc.expectedReason, data.Reason)
			}
		})
	}
}

// TestUserEventConsumer_StartAndStop validates the background consumer lifecycle.
// Why: Ensures clean startup and shutdown without hanging or dropping connection handles.
func TestUserEventConsumer_StartAndStop(t *testing.T) {
	svc := newMockOrderService()
	reader := &mockMessageReader{
		messages: []kafkaGo.Message{
			{Topic: "user.events", Value: []byte(`{"event_type":"user.deleted","data":{"user_id":"usr_bg_1"}}`)},
		},
	}
	c := consumer.NewUserEventConsumerWithReader(reader, svc, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	_ = c.Close()

	if len(svc.deletedUsers) != 1 || svc.deletedUsers[0] != "usr_bg_1" {
		t.Errorf("expected deletedUsers [usr_bg_1], got %v", svc.deletedUsers)
	}
	if !reader.closed {
		t.Error("expected reader to be closed")
	}
}
