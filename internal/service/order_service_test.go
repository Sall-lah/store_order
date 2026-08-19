package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sall-lah/store_order/internal/db"
	"github.com/Sall-lah/store_order/internal/integration/midtrans"
	"github.com/Sall-lah/store_order/internal/integration/product"
	"github.com/Sall-lah/store_order/internal/repository"
	"github.com/Sall-lah/store_order/internal/service"
)

// MockOrderRepository implements repository.OrderRepository in memory for unit testing.
type MockOrderRepository struct {
	orders       map[string]*db.OrderModel
	outboxEvents []repository.OutboxCreateInput
}

func NewMockOrderRepo() *MockOrderRepository {
	return &MockOrderRepository{
		orders: make(map[string]*db.OrderModel),
	}
}

func (m *MockOrderRepository) CreateOrderWithItemsAndOutbox(
	ctx context.Context,
	orderInput repository.OrderCreateInput,
	items []repository.OrderItemInput,
	outbox *repository.OutboxCreateInput,
) (*db.OrderModel, error) {
	orderID := "ord_mock_123"
	now := time.Now()
	order := &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:          orderID,
			OrderNumber: orderInput.OrderNumber,
			UserID:      orderInput.UserID,
			UserEmail:   orderInput.UserEmail,
			Status:      db.OrderStatusPendingPayment,
			TotalAmount: orderInput.TotalAmount,
			ShippingFee: orderInput.ShippingFee,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		RelationsOrder: db.RelationsOrder{},
	}
	m.orders[orderID] = order
	if outbox != nil {
		m.outboxEvents = append(m.outboxEvents, *outbox)
	}
	return order, nil
}

func (m *MockOrderRepository) GetOrderByID(ctx context.Context, orderID string) (*db.OrderModel, error) {
	if o, ok := m.orders[orderID]; ok {
		return o, nil
	}
	return nil, nil
}

func (m *MockOrderRepository) GetOrderByOrderNumber(ctx context.Context, orderNumber string) (*db.OrderModel, error) {
	for _, o := range m.orders {
		if o.OrderNumber == orderNumber {
			return o, nil
		}
	}
	return nil, nil
}

func (m *MockOrderRepository) ListOrdersByUserID(ctx context.Context, userID string, limit, offset int) ([]db.OrderModel, int, error) {
	var list []db.OrderModel
	for _, o := range m.orders {
		if o.UserID == userID {
			list = append(list, *o)
		}
	}
	return list, len(list), nil
}

func (m *MockOrderRepository) ListAllOrders(ctx context.Context, filter repository.OrderFilter) ([]db.OrderModel, int, error) {
	var list []db.OrderModel
	for _, o := range m.orders {
		list = append(list, *o)
	}
	return list, len(list), nil
}

func (m *MockOrderRepository) UpdateOrderStatusWithOutbox(
	ctx context.Context,
	orderID string,
	newStatus db.OrderStatus,
	meta *repository.PaymentMetadata,
	outbox *repository.OutboxCreateInput,
) (*db.OrderModel, error) {
	if o, ok := m.orders[orderID]; ok {
		o.InnerOrder.Status = newStatus
		if meta != nil && meta.PaymentType != "" {
			o.InnerOrder.PaymentType = &meta.PaymentType
		}
		if outbox != nil {
			m.outboxEvents = append(m.outboxEvents, *outbox)
		}
		return o, nil
	}
	return nil, nil
}

func (m *MockOrderRepository) UpdateSnapToken(ctx context.Context, orderID, snapToken, snapRedirectURL string) error {
	if o, ok := m.orders[orderID]; ok {
		o.InnerOrder.SnapToken = &snapToken
		o.InnerOrder.SnapRedirectURL = &snapRedirectURL
	}
	return nil
}

// MockProductClient mocks store_product validation.
type MockProductClient struct{}

func (m *MockProductClient) GetProductByID(ctx context.Context, productID string) (*product.ProductDTO, error) {
	return &product.ProductDTO{
		ID:        productID,
		Name:      "Test Product",
		BasePrice: 150000.00,
		IsActive:  true,
	}, nil
}

func (m *MockProductClient) ValidateItems(ctx context.Context, reqs []product.ItemOrderRequest) ([]product.ValidatedItem, float64, error) {
	var items []product.ValidatedItem
	var total float64
	for _, r := range reqs {
		sub := 150000.00 * float64(r.Quantity)
		total += sub
		items = append(items, product.ValidatedItem{
			ProductID:   r.ProductID,
			VariantID:   r.VariantID,
			ProductName: "Test Product",
			SKU:         "SKU-TEST",
			UnitPrice:   150000.00,
			Quantity:    r.Quantity,
			Subtotal:    sub,
		})
	}
	return items, total, nil
}

// MockMidtransClient mocks Snap token generation.
type MockMidtransClient struct{}

func (m *MockMidtransClient) CreateSnapTransaction(ctx context.Context, req midtrans.SnapTransactionRequest) (*midtrans.SnapResponse, error) {
	return &midtrans.SnapResponse{
		Token:       "mock-token-123",
		RedirectURL: "https://app.sandbox.midtrans.com/snap/v2/vtweb/mock-token-123",
	}, nil
}

func TestOrderCheckoutAndSimulationFlow(t *testing.T) {
	repo := NewMockOrderRepo()
	prodClient := &MockProductClient{}
	midClient := &MockMidtransClient{}
	svc := service.NewOrderService(repo, prodClient, midClient, "test-server-key", true)

	ctx := context.Background()

	// 1. Checkout
	resp, err := svc.Checkout(ctx, "usr_100", "customer@example.com", service.CheckoutRequest{
		Items: []product.ItemOrderRequest{
			{ProductID: "prod_1", VariantID: "var_1", Quantity: 2},
		},
		ShippingFee:     20000,
		ShippingAddress: "Jl. Sudirman No 1, Jakarta",
	})
	if err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	if resp.Status != "PENDING_PAYMENT" {
		t.Errorf("expected status PENDING_PAYMENT, got %s", resp.Status)
	}
	if resp.TotalAmount != 320000.00 { // 2*150000 + 20000
		t.Errorf("expected total 320000, got %f", resp.TotalAmount)
	}
	if resp.SnapToken != "mock-token-123" {
		t.Errorf("expected snap token mock-token-123, got %s", resp.SnapToken)
	}

	// 2. Simulate Success
	paidResp, err := svc.SimulatePaymentSuccess(ctx, resp.ID)
	if err != nil {
		t.Fatalf("simulate payment success failed: %v", err)
	}
	if paidResp.Status != "PAID" {
		t.Errorf("expected status PAID, got %s", paidResp.Status)
	}

	// Verify Outbox events recorded
	if len(repo.outboxEvents) < 2 {
		t.Errorf("expected at least 2 outbox events (order.created and order.paid), got %d", len(repo.outboxEvents))
	}
}
