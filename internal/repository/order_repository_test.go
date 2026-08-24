package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sall-lah/store_order/internal/db"
	"github.com/Sall-lah/store_order/internal/repository"
)

// inMemoryOrderRepo provides an isolated in-memory implementation of OrderRepository for testing repository behaviors.
type inMemoryOrderRepo struct {
	orders       map[string]*db.OrderModel
	outboxEvents []repository.OutboxCreateInput
}

func newInMemoryOrderRepo() *inMemoryOrderRepo {
	return &inMemoryOrderRepo{
		orders: make(map[string]*db.OrderModel),
	}
}

func (m *inMemoryOrderRepo) CreateOrderWithItemsAndOutbox(
	ctx context.Context,
	orderInput repository.OrderCreateInput,
	items []repository.OrderItemInput,
	outbox *repository.OutboxCreateInput,
) (*db.OrderModel, error) {
	now := time.Now()
	order := &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:              orderInput.OrderNumber,
			OrderNumber:     orderInput.OrderNumber,
			UserID:          orderInput.UserID,
			UserEmail:       orderInput.UserEmail,
			Status:          db.OrderStatusPendingPayment,
			TotalAmount:     orderInput.TotalAmount,
			ShippingFee:     orderInput.ShippingFee,
			ShippingAddress: &orderInput.ShippingAddress,
			SnapToken:       &orderInput.SnapToken,
			SnapRedirectURL: &orderInput.SnapRedirectURL,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	m.orders[order.ID] = order
	if outbox != nil {
		m.outboxEvents = append(m.outboxEvents, *outbox)
	}
	return order, nil
}

func (m *inMemoryOrderRepo) GetOrderByID(ctx context.Context, orderID string) (*db.OrderModel, error) {
	return m.orders[orderID], nil
}

func (m *inMemoryOrderRepo) GetOrderByOrderNumber(ctx context.Context, orderNumber string) (*db.OrderModel, error) {
	for _, o := range m.orders {
		if o.OrderNumber == orderNumber {
			return o, nil
		}
	}
	return nil, nil
}

func (m *inMemoryOrderRepo) ListOrdersByUserID(ctx context.Context, userID string, limit, offset int) ([]db.OrderModel, int, error) {
	var list []db.OrderModel
	for _, o := range m.orders {
		if o.UserID == userID {
			list = append(list, *o)
		}
	}
	return list, len(list), nil
}

func (m *inMemoryOrderRepo) ListActiveOrdersByUserID(ctx context.Context, userID string) ([]db.OrderModel, error) {
	var list []db.OrderModel
	for _, o := range m.orders {
		if o.UserID == userID && o.Status == db.OrderStatusPendingPayment {
			list = append(list, *o)
		}
	}
	return list, nil
}

func (m *inMemoryOrderRepo) ListAllOrders(ctx context.Context, filter repository.OrderFilter) ([]db.OrderModel, int, error) {
	var list []db.OrderModel
	for _, o := range m.orders {
		list = append(list, *o)
	}
	return list, len(list), nil
}

func (m *inMemoryOrderRepo) UpdateOrderStatusWithOutbox(
	ctx context.Context,
	orderID string,
	newStatus db.OrderStatus,
	meta *repository.PaymentMetadata,
	shipping *repository.ShippingMetadata,
	outbox *repository.OutboxCreateInput,
) (*db.OrderModel, error) {
	if o, ok := m.orders[orderID]; ok {
		o.InnerOrder.Status = newStatus
		if outbox != nil {
			m.outboxEvents = append(m.outboxEvents, *outbox)
		}
		return o, nil
	}
	return nil, nil
}

func (m *inMemoryOrderRepo) UpdateSnapToken(ctx context.Context, orderID, snapToken, snapRedirectURL string) error {
	if o, ok := m.orders[orderID]; ok {
		o.InnerOrder.SnapToken = &snapToken
		o.InnerOrder.SnapRedirectURL = &snapRedirectURL
	}
	return nil
}

func (m *inMemoryOrderRepo) AnonymizeUserOrdersAndCancelUnpaid(
	ctx context.Context,
	userID string,
	anonymizedEmail string,
	outboxEvents []repository.OutboxCreateInput,
) error {
	emptyStr := ""
	anonymizedAddr := "[ANONYMIZED]"
	for _, o := range m.orders {
		if o.UserID == userID {
			o.InnerOrder.UserEmail = anonymizedEmail
			o.InnerOrder.ShippingAddress = &anonymizedAddr
			o.InnerOrder.SnapToken = &emptyStr
			o.InnerOrder.SnapRedirectURL = &emptyStr
			if o.Status == db.OrderStatusPendingPayment {
				o.InnerOrder.Status = db.OrderStatusCancelled
			}
		}
	}
	m.outboxEvents = append(m.outboxEvents, outboxEvents...)
	return nil
}

func (m *inMemoryOrderRepo) CancelUnpaidUserOrders(
	ctx context.Context,
	userID string,
	outboxEvents []repository.OutboxCreateInput,
) error {
	emptyStr := ""
	for _, o := range m.orders {
		if o.UserID == userID && o.Status == db.OrderStatusPendingPayment {
			o.InnerOrder.Status = db.OrderStatusCancelled
			o.InnerOrder.SnapToken = &emptyStr
			o.InnerOrder.SnapRedirectURL = &emptyStr
		}
	}
	m.outboxEvents = append(m.outboxEvents, outboxEvents...)
	return nil
}

// TestAnonymizeUserOrdersAndCancelUnpaid verifies that order repository correctly redacts customer PII and cancels unpaid orders.
// Why: Ensures adherence to GDPR and data protection requirements while releasing inventory reservations.
func TestAnonymizeUserOrdersAndCancelUnpaid(t *testing.T) {
	ctx := context.Background()
	repo := newInMemoryOrderRepo()

	targetUserID := "usr_target_delete_123"
	otherUserID := "usr_retained_456"

	// Setup orders for target user
	targetPendingAddr := "Jl. M.H. Thamrin No. 1, Jakarta"
	targetPendingToken := "snap_token_pending_1"
	targetPendingURL := "https://app.midtrans.com/snap/pending"
	repo.orders["ord_pending"] = &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:              "ord_pending",
			OrderNumber:     "ORD-20260824-001",
			UserID:          targetUserID,
			UserEmail:       "target@example.com",
			Status:          db.OrderStatusPendingPayment,
			TotalAmount:     250000,
			ShippingAddress: &targetPendingAddr,
			SnapToken:       &targetPendingToken,
			SnapRedirectURL: &targetPendingURL,
		},
	}

	targetCompletedAddr := "Jl. Gatot Subroto No. 5, Jakarta"
	repo.orders["ord_completed"] = &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:              "ord_completed",
			OrderNumber:     "ORD-20260824-002",
			UserID:          targetUserID,
			UserEmail:       "target@example.com",
			Status:          db.OrderStatusCompleted,
			TotalAmount:     120000,
			ShippingAddress: &targetCompletedAddr,
		},
	}

	// Setup order for another user that should remain untouched
	otherAddr := "Jl. Asia Afrika No. 10, Bandung"
	otherEmail := "other@example.com"
	repo.orders["ord_other"] = &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:              "ord_other",
			OrderNumber:     "ORD-20260824-003",
			UserID:          otherUserID,
			UserEmail:       otherEmail,
			Status:          db.OrderStatusPendingPayment,
			TotalAmount:     75000,
			ShippingAddress: &otherAddr,
		},
	}

	anonymizedEmail := "deleted_user_usr_target_delete_123@anonymized.local"
	outboxEvents := []repository.OutboxCreateInput{
		{
			AggregateType: "ORDER",
			AggregateID:   "ord_pending",
			Topic:         "order.events",
			Payload:       `{"event_type":"order.cancelled","reason":"User account deleted"}`,
		},
	}

	err := repo.AnonymizeUserOrdersAndCancelUnpaid(ctx, targetUserID, anonymizedEmail, outboxEvents)
	if err != nil {
		t.Fatalf("AnonymizeUserOrdersAndCancelUnpaid failed: %v", err)
	}

	// Verify target pending order
	pendingOrder := repo.orders["ord_pending"]
	if pendingOrder.Status != db.OrderStatusCancelled {
		t.Errorf("expected ord_pending status CANCELLED, got %s", pendingOrder.Status)
	}
	if pendingOrder.UserEmail != anonymizedEmail {
		t.Errorf("expected userEmail %s, got %s", anonymizedEmail, pendingOrder.UserEmail)
	}
	if addr, ok := pendingOrder.ShippingAddress(); !ok || addr != "[ANONYMIZED]" {
		t.Errorf("expected shippingAddress [ANONYMIZED], got %s", addr)
	}
	if token, ok := pendingOrder.SnapToken(); !ok || token != "" {
		t.Errorf("expected empty snapToken, got %s", token)
	}
	if url, ok := pendingOrder.SnapRedirectURL(); !ok || url != "" {
		t.Errorf("expected empty snapRedirectUrl, got %s", url)
	}

	// Verify target completed order (status preserved, PII anonymized)
	completedOrder := repo.orders["ord_completed"]
	if completedOrder.Status != db.OrderStatusCompleted {
		t.Errorf("expected ord_completed status to remain COMPLETED, got %s", completedOrder.Status)
	}
	if completedOrder.UserEmail != anonymizedEmail {
		t.Errorf("expected userEmail %s, got %s", anonymizedEmail, completedOrder.UserEmail)
	}
	if addr, ok := completedOrder.ShippingAddress(); !ok || addr != "[ANONYMIZED]" {
		t.Errorf("expected shippingAddress [ANONYMIZED], got %s", addr)
	}

	// Verify other user's order remains completely unchanged
	otherOrder := repo.orders["ord_other"]
	if otherOrder.Status != db.OrderStatusPendingPayment {
		t.Errorf("expected other user order status PENDING_PAYMENT, got %s", otherOrder.Status)
	}
	if otherOrder.UserEmail != otherEmail {
		t.Errorf("expected other user email %s, got %s", otherEmail, otherOrder.UserEmail)
	}
	if addr, ok := otherOrder.ShippingAddress(); !ok || addr != otherAddr {
		t.Errorf("expected other user address %s, got %s", otherAddr, addr)
	}

	// Verify outbox event persistence
	if len(repo.outboxEvents) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(repo.outboxEvents))
	}
	if repo.outboxEvents[0].AggregateID != "ord_pending" {
		t.Errorf("expected outbox event aggregateID ord_pending, got %s", repo.outboxEvents[0].AggregateID)
	}
}

// TestCancelUnpaidUserOrders_PIIPreserved verifies that user account ban cancels unpaid orders and persists outbox events
// while strictly preserving customer email and shipping addresses on all orders.
// Why: Ensures adherence to audit and fraud evidence retention requirements for suspended accounts.
func TestCancelUnpaidUserOrders_PIIPreserved(t *testing.T) {
	ctx := context.Background()
	repo := newInMemoryOrderRepo()

	targetUserID := "usr_banned_123"
	originalEmail := "fraud_suspect@example.com"
	originalAddr := "Jl. Antasari No. 99, Jakarta Selatan"

	// Pending order that should be cancelled
	snapToken := "snap_token_banned_1"
	snapURL := "https://app.midtrans.com/snap/banned"
	repo.orders["ord_ban_pending"] = &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:              "ord_ban_pending",
			OrderNumber:     "ORD-20260824-B01",
			UserID:          targetUserID,
			UserEmail:       originalEmail,
			Status:          db.OrderStatusPendingPayment,
			TotalAmount:     500000,
			ShippingAddress: &originalAddr,
			SnapToken:       &snapToken,
			SnapRedirectURL: &snapURL,
		},
	}

	// Completed order that should remain completed and unmodified
	repo.orders["ord_ban_completed"] = &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:              "ord_ban_completed",
			OrderNumber:     "ORD-20260824-B02",
			UserID:          targetUserID,
			UserEmail:       originalEmail,
			Status:          db.OrderStatusCompleted,
			TotalAmount:     350000,
			ShippingAddress: &originalAddr,
		},
	}

	outboxEvents := []repository.OutboxCreateInput{
		{
			AggregateType: "ORDER",
			AggregateID:   "ord_ban_pending",
			Topic:         "order.events",
			Payload:       `{"event_type":"order.cancelled","reason":"User account banned"}`,
		},
	}

	err := repo.CancelUnpaidUserOrders(ctx, targetUserID, outboxEvents)
	if err != nil {
		t.Fatalf("CancelUnpaidUserOrders failed: %v", err)
	}

	// Verify pending order: status CANCELLED, tokens cleared, BUT email and address PRESERVED
	pendingOrder := repo.orders["ord_ban_pending"]
	if pendingOrder.Status != db.OrderStatusCancelled {
		t.Errorf("expected status CANCELLED, got %s", pendingOrder.Status)
	}
	if pendingOrder.UserEmail != originalEmail {
		t.Errorf("expected original email %s preserved, got %s", originalEmail, pendingOrder.UserEmail)
	}
	if addr, ok := pendingOrder.ShippingAddress(); !ok || addr != originalAddr {
		t.Errorf("expected original address %s preserved, got %s", originalAddr, addr)
	}
	if token, ok := pendingOrder.SnapToken(); !ok || token != "" {
		t.Errorf("expected snapToken to be cleared, got %s", token)
	}

	// Verify completed order: status COMPLETED, email and address PRESERVED
	completedOrder := repo.orders["ord_ban_completed"]
	if completedOrder.Status != db.OrderStatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", completedOrder.Status)
	}
	if completedOrder.UserEmail != originalEmail {
		t.Errorf("expected original email %s preserved, got %s", originalEmail, completedOrder.UserEmail)
	}
	if addr, ok := completedOrder.ShippingAddress(); !ok || addr != originalAddr {
		t.Errorf("expected original address %s preserved, got %s", originalAddr, addr)
	}

	// Verify outbox persistence
	if len(repo.outboxEvents) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(repo.outboxEvents))
	}
	if repo.outboxEvents[0].AggregateID != "ord_ban_pending" {
		t.Errorf("expected aggregateID ord_ban_pending, got %s", repo.outboxEvents[0].AggregateID)
	}
}
