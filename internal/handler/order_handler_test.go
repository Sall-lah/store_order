package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sall-lah/store_order/internal/config"
	"github.com/Sall-lah/store_order/internal/db"
	"github.com/Sall-lah/store_order/internal/handler"
	"github.com/Sall-lah/store_order/internal/integration/midtrans"
	"github.com/Sall-lah/store_order/internal/integration/product"
	"github.com/Sall-lah/store_order/internal/repository"
	"github.com/Sall-lah/store_order/internal/router"
	"github.com/Sall-lah/store_order/internal/service"
)

// MockServiceOrderRepo for handler integration tests
type MockServiceOrderRepo struct {
	orders map[string]*db.OrderModel
}

func (m *MockServiceOrderRepo) CreateOrderWithItemsAndOutbox(
	ctx context.Context,
	orderInput repository.OrderCreateInput,
	items []repository.OrderItemInput,
	outbox *repository.OutboxCreateInput,
) (*db.OrderModel, error) {
	order := &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:          "ord_test_999",
			OrderNumber: orderInput.OrderNumber,
			UserID:      orderInput.UserID,
			UserEmail:   orderInput.UserEmail,
			Status:      db.OrderStatusPendingPayment,
			TotalAmount: orderInput.TotalAmount,
		},
	}
	m.orders[order.ID] = order
	return order, nil
}

func (m *MockServiceOrderRepo) GetOrderByID(ctx context.Context, orderID string) (*db.OrderModel, error) {
	if o, ok := m.orders[orderID]; ok {
		return o, nil
	}
	return nil, nil
}

func (m *MockServiceOrderRepo) GetOrderByOrderNumber(ctx context.Context, orderNumber string) (*db.OrderModel, error) {
	for _, o := range m.orders {
		if o.OrderNumber == orderNumber {
			return o, nil
		}
	}
	return nil, nil
}

func (m *MockServiceOrderRepo) ListOrdersByUserID(ctx context.Context, userID string, limit, offset int) ([]db.OrderModel, int, error) {
	var res []db.OrderModel
	for _, o := range m.orders {
		if o.UserID == userID {
			res = append(res, *o)
		}
	}
	return res, len(res), nil
}

func (m *MockServiceOrderRepo) ListAllOrders(ctx context.Context, filter repository.OrderFilter) ([]db.OrderModel, int, error) {
	var res []db.OrderModel
	for _, o := range m.orders {
		res = append(res, *o)
	}
	return res, len(res), nil
}

func (m *MockServiceOrderRepo) UpdateOrderStatusWithOutbox(
	ctx context.Context,
	orderID string,
	newStatus db.OrderStatus,
	meta *repository.PaymentMetadata,
	outbox *repository.OutboxCreateInput,
) (*db.OrderModel, error) {
	if o, ok := m.orders[orderID]; ok {
		o.Status = newStatus
		return o, nil
	}
	return nil, nil
}

func (m *MockServiceOrderRepo) UpdateSnapToken(ctx context.Context, orderID, snapToken, snapRedirectURL string) error {
	return nil
}

type MockHandlerProductClient struct{}

func (m *MockHandlerProductClient) GetProductByID(ctx context.Context, id string) (*product.ProductDTO, error) {
	return &product.ProductDTO{ID: id, Name: "Item", BasePrice: 100000, IsActive: true}, nil
}

func (m *MockHandlerProductClient) ValidateItems(ctx context.Context, reqs []product.ItemOrderRequest) ([]product.ValidatedItem, float64, error) {
	return []product.ValidatedItem{
		{ProductID: "p1", VariantID: "v1", ProductName: "Item", SKU: "SKU1", UnitPrice: 100000, Quantity: 1, Subtotal: 100000},
	}, 100000, nil
}

type MockHandlerMidtransClient struct{}

func (m *MockHandlerMidtransClient) CreateSnapTransaction(ctx context.Context, req midtrans.SnapTransactionRequest) (*midtrans.SnapResponse, error) {
	return &midtrans.SnapResponse{Token: "tok_123", RedirectURL: "https://midtrans.test/redirect"}, nil
}

func TestCheckoutHTTPHandler(t *testing.T) {
	repo := &MockServiceOrderRepo{orders: make(map[string]*db.OrderModel)}
	svc := service.NewOrderService(repo, &MockHandlerProductClient{}, &MockHandlerMidtransClient{}, "secret", true)

	cfg := &config.Config{Dev: true, EnableDocs: true}
	r := router.SetupRouter(router.RouterDeps{
		Config:         cfg,
		OrderHandler:   handler.NewOrderHandler(svc),
		WebhookHandler: handler.NewWebhookHandler(svc),
		AdminHandler:   handler.NewAdminHandler(svc),
		DevHandler:     handler.NewDevHandler(svc),
		HealthHandler:  handler.NewHealthHandler(nil),
	})

	// 1. Unauthenticated Checkout -> 401
	reqBody, _ := json.Marshal(service.CheckoutRequest{
		Items: []product.ItemOrderRequest{{ProductID: "p1", VariantID: "v1", Quantity: 1}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 2. Authenticated Checkout with X-User-Id -> 201
	authReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBuffer(reqBody))
	authReq.Header.Set("Content-Type", "application/json")
	authReq.Header.Set("X-User-Id", "usr_customer_1")
	authReq.Header.Set("X-User-Email", "customer@test.com")
	authRec := httptest.NewRecorder()
	r.ServeHTTP(authRec, authReq)

	if authRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d (body: %s)", authRec.Code, authRec.Body.String())
	}

	var orderResp service.OrderResponse
	if err := json.NewDecoder(authRec.Body).Decode(&orderResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if orderResp.UserID != "usr_customer_1" {
		t.Errorf("expected user ID usr_customer_1, got %s", orderResp.UserID)
	}

	// 3. Dev Simulation Endpoint (Simulate Success)
	simReq := httptest.NewRequest(http.MethodPost, "/api/v1/dev/orders/"+orderResp.ID+"/simulate-success", nil)
	simRec := httptest.NewRecorder()
	r.ServeHTTP(simRec, simReq)

	if simRec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for dev simulation, got %d (body: %s)", simRec.Code, simRec.Body.String())
	}
}

func TestHealthCheck(t *testing.T) {
	cfg := &config.Config{Dev: false}
	r := router.SetupRouter(router.RouterDeps{
		Config:        cfg,
		HealthHandler: handler.NewHealthHandler(nil),
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
}
