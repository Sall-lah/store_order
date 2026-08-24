package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sall-lah/store_order/internal/db"
	"github.com/Sall-lah/store_order/internal/repository"
	orderv1 "github.com/Sall-lah/store_proto/gen/go/store/order/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockOrderRepo implements repository.OrderRepository for gRPC unit tests.
type mockOrderRepo struct {
	activeOrders []db.OrderModel
	err          error
}

func (m *mockOrderRepo) CreateOrderWithItemsAndOutbox(ctx context.Context, orderInput repository.OrderCreateInput, items []repository.OrderItemInput, outbox *repository.OutboxCreateInput) (*db.OrderModel, error) {
	return nil, errors.New("not implemented")
}

func (m *mockOrderRepo) GetOrderByID(ctx context.Context, orderID string) (*db.OrderModel, error) {
	return nil, errors.New("not implemented")
}

func (m *mockOrderRepo) GetOrderByOrderNumber(ctx context.Context, orderNumber string) (*db.OrderModel, error) {
	return nil, errors.New("not implemented")
}

func (m *mockOrderRepo) ListOrdersByUserID(ctx context.Context, userID string, limit, offset int) ([]db.OrderModel, int, error) {
	return nil, 0, errors.New("not implemented")
}

func (m *mockOrderRepo) ListActiveOrdersByUserID(ctx context.Context, userID string) ([]db.OrderModel, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.activeOrders, nil
}

func (m *mockOrderRepo) ListAllOrders(ctx context.Context, filter repository.OrderFilter) ([]db.OrderModel, int, error) {
	return nil, 0, errors.New("not implemented")
}

func (m *mockOrderRepo) UpdateOrderStatusWithOutbox(ctx context.Context, orderID string, newStatus db.OrderStatus, meta *repository.PaymentMetadata, shipping *repository.ShippingMetadata, outbox *repository.OutboxCreateInput) (*db.OrderModel, error) {
	return nil, errors.New("not implemented")
}

func (m *mockOrderRepo) UpdateSnapToken(ctx context.Context, orderID, snapToken, snapRedirectURL string) error {
	return errors.New("not implemented")
}

func (m *mockOrderRepo) AnonymizeUserOrdersAndCancelUnpaid(ctx context.Context, userID string, anonymizedEmail string, outboxEvents []repository.OutboxCreateInput) error {
	return errors.New("not implemented")
}

func (m *mockOrderRepo) CancelUnpaidUserOrders(ctx context.Context, userID string, outboxEvents []repository.OutboxCreateInput) error {
	return errors.New("not implemented")
}

// TestStatusMappers tests bidirectional mapping between database order status and protobuf enum values.
func TestStatusMappers(t *testing.T) {
	cases := []struct {
		dbStatus    db.OrderStatus
		protoStatus orderv1.OrderStatus
	}{
		{db.OrderStatusPendingPayment, orderv1.OrderStatus_ORDER_STATUS_PENDING_PAYMENT},
		{db.OrderStatusPaid, orderv1.OrderStatus_ORDER_STATUS_PAID},
		{db.OrderStatusProcessing, orderv1.OrderStatus_ORDER_STATUS_PROCESSING},
		{db.OrderStatusShipped, orderv1.OrderStatus_ORDER_STATUS_SHIPPED},
		{db.OrderStatusCompleted, orderv1.OrderStatus_ORDER_STATUS_COMPLETED},
		{db.OrderStatusCancelled, orderv1.OrderStatus_ORDER_STATUS_CANCELLED},
		{db.OrderStatusExpired, orderv1.OrderStatus_ORDER_STATUS_EXPIRED},
	}

	for _, tc := range cases {
		t.Run(string(tc.dbStatus), func(t *testing.T) {
			gotProto := MapDBStatusToProto(tc.dbStatus)
			if gotProto != tc.protoStatus {
				t.Errorf("MapDBStatusToProto(%s) = %v; want %v", tc.dbStatus, gotProto, tc.protoStatus)
			}

			gotDB, ok := MapProtoStatusToDB(tc.protoStatus)
			if !ok || gotDB != tc.dbStatus {
				t.Errorf("MapProtoStatusToDB(%v) = (%s, %v); want (%s, true)", tc.protoStatus, gotDB, ok, tc.dbStatus)
			}
		})
	}

	// Unknown enum fallback
	if MapDBStatusToProto("UNKNOWN_STATUS") != orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		t.Errorf("Expected ORDER_STATUS_UNSPECIFIED for unknown DB status")
	}

	if _, ok := MapProtoStatusToDB(orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED); ok {
		t.Errorf("Expected false for ORDER_STATUS_UNSPECIFIED in MapProtoStatusToDB")
	}
}

// TestMapOrderModelToActiveSummary validates the formatting and projection of an OrderModel into ActiveOrderSummary.
func TestMapOrderModelToActiveSummary(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	order := db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:          "ord_12345",
			OrderNumber: "ORD-2026-001",
			Status:      db.OrderStatusPaid,
			TotalAmount: 150000.0,
			CreatedAt:   now,
		},
	}

	summary := MapOrderModelToActiveSummary(order)
	if summary.OrderId != "ord_12345" {
		t.Errorf("Expected OrderId ord_12345, got %s", summary.OrderId)
	}
	if summary.OrderNumber != "ORD-2026-001" {
		t.Errorf("Expected OrderNumber ORD-2026-001, got %s", summary.OrderNumber)
	}
	if summary.Status != orderv1.OrderStatus_ORDER_STATUS_PAID {
		t.Errorf("Expected Status PAID, got %v", summary.Status)
	}
	if summary.TotalAmount != 150000.0 {
		t.Errorf("Expected TotalAmount 150000.0, got %f", summary.TotalAmount)
	}
	if summary.CreatedAt != now.Format(time.RFC3339) {
		t.Errorf("Expected CreatedAt %s, got %s", now.Format(time.RFC3339), summary.CreatedAt)
	}
}

// TestCheckActiveOrders tests all scenarios of the CheckActiveOrders gRPC method.
func TestCheckActiveOrders(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("invalid arguments - empty user ID", func(t *testing.T) {
		server := NewOrderServiceServer(&mockOrderRepo{})

		// nil request
		_, err := server.CheckActiveOrders(ctx, nil)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument for nil request, got %v", err)
		}

		// empty user ID
		_, err = server.CheckActiveOrders(ctx, &orderv1.CheckActiveOrdersRequest{UserId: "   "})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument for blank user ID, got %v", err)
		}
	})

	t.Run("user has active orders", func(t *testing.T) {
		repo := &mockOrderRepo{
			activeOrders: []db.OrderModel{
				{
					InnerOrder: db.InnerOrder{
						ID:          "ord_1",
						OrderNumber: "ORD-001",
						Status:      db.OrderStatusPendingPayment,
						TotalAmount: 50000,
						CreatedAt:   now,
					},
				},
				{
					InnerOrder: db.InnerOrder{
						ID:          "ord_2",
						OrderNumber: "ORD-002",
						Status:      db.OrderStatusProcessing,
						TotalAmount: 75000,
						CreatedAt:   now,
					},
				},
			},
		}
		server := NewOrderServiceServer(repo)

		resp, err := server.CheckActiveOrders(ctx, &orderv1.CheckActiveOrdersRequest{UserId: "usr_active"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !resp.HasActiveOrders {
			t.Errorf("Expected HasActiveOrders true, got false")
		}
		if resp.ActiveOrderCount != 2 {
			t.Errorf("Expected ActiveOrderCount 2, got %d", resp.ActiveOrderCount)
		}
		if len(resp.ActiveOrders) != 2 {
			t.Errorf("Expected 2 ActiveOrders summaries, got %d", len(resp.ActiveOrders))
		}
		if resp.ActiveOrders[0].OrderId != "ord_1" || resp.ActiveOrders[0].Status != orderv1.OrderStatus_ORDER_STATUS_PENDING_PAYMENT {
			t.Errorf("Order 0 mismatch: %+v", resp.ActiveOrders[0])
		}
		if resp.ActiveOrders[1].OrderId != "ord_2" || resp.ActiveOrders[1].Status != orderv1.OrderStatus_ORDER_STATUS_PROCESSING {
			t.Errorf("Order 1 mismatch: %+v", resp.ActiveOrders[1])
		}
	})

	t.Run("user has no active orders", func(t *testing.T) {
		repo := &mockOrderRepo{
			activeOrders: []db.OrderModel{},
		}
		server := NewOrderServiceServer(repo)

		resp, err := server.CheckActiveOrders(ctx, &orderv1.CheckActiveOrdersRequest{UserId: "usr_empty"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if resp.HasActiveOrders {
			t.Errorf("Expected HasActiveOrders false, got true")
		}
		if resp.ActiveOrderCount != 0 {
			t.Errorf("Expected ActiveOrderCount 0, got %d", resp.ActiveOrderCount)
		}
		if len(resp.ActiveOrders) != 0 {
			t.Errorf("Expected 0 ActiveOrders summaries, got %d", len(resp.ActiveOrders))
		}
		if resp.Message != "User has no active orders" {
			t.Errorf("Unexpected message: %s", resp.Message)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &mockOrderRepo{
			err: errors.New("db connection failure"),
		}
		server := NewOrderServiceServer(repo)

		_, err := server.CheckActiveOrders(ctx, &orderv1.CheckActiveOrdersRequest{UserId: "usr_err"})
		if status.Code(err) != codes.Internal {
			t.Errorf("Expected Internal status code, got %v", status.Code(err))
		}
	})
}
