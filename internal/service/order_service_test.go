package service_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	var orderItems []db.OrderItemModel
	for i, it := range items {
		vName := it.VariantName
		orderItems = append(orderItems, db.OrderItemModel{
			InnerOrderItem: db.InnerOrderItem{
				ID:          fmt.Sprintf("item_mock_%d", i+1),
				OrderID:     orderID,
				ProductID:   it.ProductID,
				VariantID:   it.VariantID,
				ProductName: it.ProductName,
				VariantName: &vName,
				Sku:         it.SKU,
				Price:       it.Price,
				Quantity:    it.Quantity,
				Subtotal:    it.Subtotal,
				CreatedAt:   now,
			},
		})
	}
	order := &db.OrderModel{
		InnerOrder: db.InnerOrder{
			ID:              orderID,
			OrderNumber:     orderInput.OrderNumber,
			UserID:          orderInput.UserID,
			UserEmail:       orderInput.UserEmail,
			Status:          db.OrderStatusPendingPayment,
			TotalAmount:     orderInput.TotalAmount,
			ShippingFee:     orderInput.ShippingFee,
			SnapToken:       &orderInput.SnapToken,
			SnapRedirectURL: &orderInput.SnapRedirectURL,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		RelationsOrder: db.RelationsOrder{
			Items: orderItems,
		},
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

func (m *MockOrderRepository) ListActiveOrdersByUserID(ctx context.Context, userID string) ([]db.OrderModel, error) {
	var list []db.OrderModel
	activeStatuses := map[db.OrderStatus]bool{
		db.OrderStatusPendingPayment: true,
		db.OrderStatusPaid:           true,
		db.OrderStatusProcessing:     true,
		db.OrderStatusShipped:        true,
	}
	for _, o := range m.orders {
		if o.UserID == userID && activeStatuses[o.Status] {
			list = append(list, *o)
		}
	}
	return list, nil
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
	shipping *repository.ShippingMetadata,
	outbox *repository.OutboxCreateInput,
) (*db.OrderModel, error) {
	if o, ok := m.orders[orderID]; ok {
		o.InnerOrder.Status = newStatus
		if meta != nil && meta.PaymentType != "" {
			o.InnerOrder.PaymentType = &meta.PaymentType
		}
		if shipping != nil {
			if shipping.CourierName != "" {
				o.InnerOrder.CourierName = &shipping.CourierName
			}
			if shipping.ReceiptNumber != "" {
				o.InnerOrder.ReceiptNumber = &shipping.ReceiptNumber
			}
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
	if resp.SnapRedirectURL != "https://app.sandbox.midtrans.com/snap/v2/vtweb/mock-token-123" {
		t.Errorf("expected snap redirect url, got %s", resp.SnapRedirectURL)
	}

	// 2. Simulate Success
	paidResp, err := svc.SimulatePaymentSuccess(ctx, resp.ID)
	if err != nil {
		t.Fatalf("simulate payment success failed: %v", err)
	}
	if paidResp.Status != "PAID" {
		t.Errorf("expected status PAID, got %s", paidResp.Status)
	}

	// 3. Admin Update Status to SHIPPED with tracking info
	shippedResp, err := svc.AdminUpdateStatus(ctx, resp.ID, "SHIPPED", "JNE Express", "JNE-88219038")
	if err != nil {
		t.Fatalf("admin update status to SHIPPED failed: %v", err)
	}
	if shippedResp.Status != "SHIPPED" {
		t.Errorf("expected status SHIPPED, got %s", shippedResp.Status)
	}
	if shippedResp.CourierName != "JNE Express" {
		t.Errorf("expected courierName JNE Express, got %s", shippedResp.CourierName)
	}
	if shippedResp.ReceiptNumber != "JNE-88219038" {
		t.Errorf("expected receiptNumber JNE-88219038, got %s", shippedResp.ReceiptNumber)
	}

	// Verify Outbox events recorded
	if len(repo.outboxEvents) < 3 {
		t.Errorf("expected at least 3 outbox events (order.created, order.paid, order.shipped), got %d", len(repo.outboxEvents))
	}
}

// TestOrderCancellationAndExpiration_E2E tests end-to-end user cancellation and Midtrans payment expiration flows.
// Why: Guarantees that order state transitions and outbound domain event payloads contain accurate email recipient and invoice metadata for downstream notification workers.
func TestOrderCancellationAndExpiration_E2E(t *testing.T) {
	repo := NewMockOrderRepo()
	prodClient := &MockProductClient{}
	midClient := &MockMidtransClient{}
	serverKey := "SB-Mid-server-TESTKEY123"
	svc := service.NewOrderService(repo, prodClient, midClient, serverKey, true)
	ctx := context.Background()

	customerEmail := "hellofliqy1@gmail.com"
	customerUserID := "usr_hellofliqy1_id"

	t.Run("User Cancellation Flow", func(t *testing.T) {
		// 1. Customer initiates checkout
		checkoutResp, err := svc.Checkout(ctx, customerUserID, customerEmail, service.CheckoutRequest{
			Items: []product.ItemOrderRequest{
				{ProductID: "prod_jacket", VariantID: "var_jacket_m", Quantity: 1},
			},
			ShippingFee:     25000,
			ShippingAddress: "Jl. Sudirman No. 45, Jakarta Selatan",
		})
		if err != nil {
			t.Fatalf("checkout failed: %v", err)
		}
		if checkoutResp.Status != "PENDING_PAYMENT" {
			t.Fatalf("expected PENDING_PAYMENT, got %s", checkoutResp.Status)
		}

		// 2. Customer explicitly cancels their pending order
		cancelResp, err := svc.CancelCustomerOrder(ctx, customerUserID, checkoutResp.ID)
		if err != nil {
			t.Fatalf("cancel order failed: %v", err)
		}
		if cancelResp.Status != "CANCELLED" {
			t.Fatalf("expected status CANCELLED, got %s", cancelResp.Status)
		}

		// 3. Verify emitted outbox event for notification worker
		if len(repo.outboxEvents) == 0 {
			t.Fatal("expected outbox event to be recorded")
		}
		lastOutbox := repo.outboxEvents[len(repo.outboxEvents)-1]
		if lastOutbox.Topic != service.TopicOrderEvents {
			t.Errorf("expected topic %s, got %s", service.TopicOrderEvents, lastOutbox.Topic)
		}

		var envelope service.DomainEventEnvelope
		if err := json.Unmarshal([]byte(lastOutbox.Payload), &envelope); err != nil {
			t.Fatalf("failed to decode domain event envelope: %v", err)
		}
		if envelope.EventType != service.EventTypeOrderCancelled {
			t.Errorf("expected event_type %s, got %s", service.EventTypeOrderCancelled, envelope.EventType)
		}

		dataMap, ok := envelope.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected envelope data to be map, got %T", envelope.Data)
		}
		if dataMap["userEmail"] != customerEmail {
			t.Errorf("expected userEmail %s, got %v", customerEmail, dataMap["userEmail"])
		}
		if dataMap["reason"] != "Customer cancelled" {
			t.Errorf("expected reason 'Customer cancelled', got %v", dataMap["reason"])
		}
	})

	t.Run("Midtrans Expiration Webhook Flow", func(t *testing.T) {
		// 1. Customer initiates checkout
		checkoutResp, err := svc.Checkout(ctx, customerUserID, customerEmail, service.CheckoutRequest{
			Items: []product.ItemOrderRequest{
				{ProductID: "prod_tee", VariantID: "var_tee_l", Quantity: 2},
			},
			ShippingFee:     15000,
			ShippingAddress: "Jl. Gatot Subroto No. 10, Jakarta Pusat",
		})
		if err != nil {
			t.Fatalf("checkout failed: %v", err)
		}

		// 2. Midtrans dispatches expiration webhook callback
		webhookNotif := midtrans.WebhookNotification{
			OrderID:           checkoutResp.OrderNumber,
			StatusCode:        "202",
			GrossAmount:       fmt.Sprintf("%.2f", checkoutResp.TotalAmount),
			TransactionStatus: "expire",
			TransactionID:     "midtrans_tx_exp_5566",
			PaymentType:       "qris",
		}
		webhookNotif.SignatureKey = midtrans.GenerateSignature(webhookNotif.OrderID, webhookNotif.StatusCode, webhookNotif.GrossAmount, serverKey)

		if err := svc.ProcessMidtransWebhook(ctx, webhookNotif); err != nil {
			t.Fatalf("webhook processing failed: %v", err)
		}

		// 3. Verify order status transitioned to EXPIRED in database
		expiredOrder, err := repo.GetOrderByOrderNumber(ctx, checkoutResp.OrderNumber)
		if err != nil || expiredOrder == nil {
			t.Fatalf("failed to retrieve updated order: %v", err)
		}
		if expiredOrder.Status != db.OrderStatusExpired {
			t.Errorf("expected status EXPIRED, got %s", expiredOrder.Status)
		}

		// 4. Verify emitted outbox event for notification worker
		lastOutbox := repo.outboxEvents[len(repo.outboxEvents)-1]
		var envelope service.DomainEventEnvelope
		if err := json.Unmarshal([]byte(lastOutbox.Payload), &envelope); err != nil {
			t.Fatalf("failed to decode domain event envelope: %v", err)
		}
		if envelope.EventType != service.EventTypeOrderExpired {
			t.Errorf("expected event_type %s, got %s", service.EventTypeOrderExpired, envelope.EventType)
		}

		dataMap, ok := envelope.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected envelope data to be map, got %T", envelope.Data)
		}
		if dataMap["userEmail"] != customerEmail {
			t.Errorf("expected userEmail %s, got %v", customerEmail, dataMap["userEmail"])
		}
		if dataMap["reason"] != "Payment expired" {
			t.Errorf("expected reason 'Payment expired', got %v", dataMap["reason"])
		}
	})
}

// TestOrderCancellation_OutboxEventSchema verifies that all cancellation triggers emit compliant OrderEventData schemas.
// Why: Ensures compatibility with store_notification service HandleOrderCancelled consumer contract and downstream inventory restock listeners.
func TestOrderCancellation_OutboxEventSchema(t *testing.T) {
	ctx := context.Background()

	t.Run("Admin Cancellation Schema", func(t *testing.T) {
		repo := NewMockOrderRepo()
		svc := service.NewOrderService(repo, &MockProductClient{}, &MockMidtransClient{}, "test-key", true)

		checkoutResp, err := svc.Checkout(ctx, "usr_admin_cancel", "admin_target@example.com", service.CheckoutRequest{
			Items: []product.ItemOrderRequest{
				{ProductID: "prod_1", VariantID: "var_1", Quantity: 3},
			},
			ShippingFee:     10000,
			ShippingAddress: "Admin Test Address",
		})
		if err != nil {
			t.Fatalf("checkout failed: %v", err)
		}

		adminResp, err := svc.AdminUpdateStatus(ctx, checkoutResp.ID, "CANCELLED", "", "")
		if err != nil {
			t.Fatalf("admin cancel failed: %v", err)
		}
		if adminResp.Status != "CANCELLED" {
			t.Errorf("expected status CANCELLED, got %s", adminResp.Status)
		}

		lastOutbox := repo.outboxEvents[len(repo.outboxEvents)-1]
		var envelope service.DomainEventEnvelope
		if err := json.Unmarshal([]byte(lastOutbox.Payload), &envelope); err != nil {
			t.Fatalf("failed to unmarshal envelope: %v", err)
		}

		if envelope.EventType != service.EventTypeOrderCancelled {
			t.Errorf("expected event_type %s, got %s", service.EventTypeOrderCancelled, envelope.EventType)
		}
		if envelope.Producer != "store_order" {
			t.Errorf("expected producer store_order, got %s", envelope.Producer)
		}

		dataMap, ok := envelope.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map data, got %T", envelope.Data)
		}

		if dataMap["orderNumber"] != checkoutResp.OrderNumber {
			t.Errorf("expected orderNumber %s, got %v", checkoutResp.OrderNumber, dataMap["orderNumber"])
		}
		if dataMap["userEmail"] != "admin_target@example.com" {
			t.Errorf("expected userEmail admin_target@example.com, got %v", dataMap["userEmail"])
		}
		if dataMap["reason"] != "Admin cancelled" {
			t.Errorf("expected reason 'Admin cancelled', got %v", dataMap["reason"])
		}
		if dataMap["status"] != "CANCELLED" {
			t.Errorf("expected status CANCELLED, got %v", dataMap["status"])
		}

		items, ok := dataMap["items"].([]interface{})
		if !ok || len(items) != 1 {
			t.Fatalf("expected 1 line item in event data, got %v", dataMap["items"])
		}
		firstItem := items[0].(map[string]interface{})
		if firstItem["productName"] != "Test Product" {
			t.Errorf("expected productName 'Test Product', got %v", firstItem["productName"])
		}
		if firstItem["quantity"].(float64) != 3 {
			t.Errorf("expected quantity 3, got %v", firstItem["quantity"])
		}
	})

	t.Run("Midtrans Deny Webhook Schema", func(t *testing.T) {
		repo := NewMockOrderRepo()
		serverKey := "SB-Mid-server-TESTKEY123"
		svc := service.NewOrderService(repo, &MockProductClient{}, &MockMidtransClient{}, serverKey, true)

		checkoutResp, err := svc.Checkout(ctx, "usr_deny", "deny_customer@example.com", service.CheckoutRequest{
			Items: []product.ItemOrderRequest{
				{ProductID: "prod_2", VariantID: "var_2", Quantity: 1},
			},
			ShippingFee:     5000,
			ShippingAddress: "Deny Test Address",
		})
		if err != nil {
			t.Fatalf("checkout failed: %v", err)
		}

		notif := midtrans.WebhookNotification{
			OrderID:           checkoutResp.OrderNumber,
			StatusCode:        "202",
			GrossAmount:       fmt.Sprintf("%.2f", checkoutResp.TotalAmount),
			TransactionStatus: "deny",
			TransactionID:     "midtrans_tx_deny_999",
			PaymentType:       "credit_card",
		}
		notif.SignatureKey = midtrans.GenerateSignature(notif.OrderID, notif.StatusCode, notif.GrossAmount, serverKey)

		if err := svc.ProcessMidtransWebhook(ctx, notif); err != nil {
			t.Fatalf("webhook process failed: %v", err)
		}

		lastOutbox := repo.outboxEvents[len(repo.outboxEvents)-1]
		var envelope service.DomainEventEnvelope
		if err := json.Unmarshal([]byte(lastOutbox.Payload), &envelope); err != nil {
			t.Fatalf("failed to unmarshal envelope: %v", err)
		}

		if envelope.EventType != service.EventTypeOrderCancelled {
			t.Errorf("expected event_type %s, got %s", service.EventTypeOrderCancelled, envelope.EventType)
		}
		dataMap := envelope.Data.(map[string]interface{})
		if dataMap["reason"] != "Midtrans transaction cancelled or denied" {
			t.Errorf("expected reason 'Midtrans transaction cancelled or denied', got %v", dataMap["reason"])
		}
		if dataMap["userEmail"] != "deny_customer@example.com" {
			t.Errorf("expected userEmail deny_customer@example.com, got %v", dataMap["userEmail"])
		}
	})

	t.Run("Dev Simulation Cancel Schema", func(t *testing.T) {
		repo := NewMockOrderRepo()
		svc := service.NewOrderService(repo, &MockProductClient{}, &MockMidtransClient{}, "test-key", true)

		checkoutResp, err := svc.Checkout(ctx, "usr_sim_cancel", "sim_cancel@example.com", service.CheckoutRequest{
			Items: []product.ItemOrderRequest{
				{ProductID: "prod_3", VariantID: "var_3", Quantity: 1},
			},
			ShippingFee:     0,
			ShippingAddress: "Simulation Address",
		})
		if err != nil {
			t.Fatalf("checkout failed: %v", err)
		}

		simResp, err := svc.SimulateOrderCancel(ctx, checkoutResp.ID)
		if err != nil {
			t.Fatalf("simulate cancel failed: %v", err)
		}
		if simResp.Status != "CANCELLED" {
			t.Errorf("expected status CANCELLED, got %s", simResp.Status)
		}

		lastOutbox := repo.outboxEvents[len(repo.outboxEvents)-1]
		var envelope service.DomainEventEnvelope
		_ = json.Unmarshal([]byte(lastOutbox.Payload), &envelope)
		dataMap := envelope.Data.(map[string]interface{})
		if dataMap["reason"] != "Customer cancelled" {
			t.Errorf("expected reason 'Customer cancelled', got %v", dataMap["reason"])
		}
	})
}

// TestOrderExpiration_OutboxEventSchema verifies that all expiration triggers emit compliant OrderEventData schemas.
// Why: Ensures compatibility with store_notification service HandleOrderExpired consumer contract and inventory replenishment.
func TestOrderExpiration_OutboxEventSchema(t *testing.T) {
	ctx := context.Background()

	t.Run("Dev Simulation Expire Schema", func(t *testing.T) {
		repo := NewMockOrderRepo()
		svc := service.NewOrderService(repo, &MockProductClient{}, &MockMidtransClient{}, "test-key", true)

		checkoutResp, err := svc.Checkout(ctx, "usr_sim_exp", "sim_expire@example.com", service.CheckoutRequest{
			Items: []product.ItemOrderRequest{
				{ProductID: "prod_4", VariantID: "var_4", Quantity: 2},
			},
			ShippingFee:     12000,
			ShippingAddress: "Simulation Expire Address",
		})
		if err != nil {
			t.Fatalf("checkout failed: %v", err)
		}

		simResp, err := svc.SimulateOrderExpire(ctx, checkoutResp.ID)
		if err != nil {
			t.Fatalf("simulate expire failed: %v", err)
		}
		if simResp.Status != "EXPIRED" {
			t.Errorf("expected status EXPIRED, got %s", simResp.Status)
		}

		lastOutbox := repo.outboxEvents[len(repo.outboxEvents)-1]
		var envelope service.DomainEventEnvelope
		if err := json.Unmarshal([]byte(lastOutbox.Payload), &envelope); err != nil {
			t.Fatalf("failed to unmarshal envelope: %v", err)
		}

		if envelope.EventType != service.EventTypeOrderExpired {
			t.Errorf("expected event_type %s, got %s", service.EventTypeOrderExpired, envelope.EventType)
		}
		if envelope.Producer != "store_order" {
			t.Errorf("expected producer store_order, got %s", envelope.Producer)
		}

		dataMap, ok := envelope.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map data, got %T", envelope.Data)
		}

		if dataMap["orderNumber"] != checkoutResp.OrderNumber {
			t.Errorf("expected orderNumber %s, got %v", checkoutResp.OrderNumber, dataMap["orderNumber"])
		}
		if dataMap["userEmail"] != "sim_expire@example.com" {
			t.Errorf("expected userEmail sim_expire@example.com, got %v", dataMap["userEmail"])
		}
		if dataMap["reason"] != "Dev simulation expired" {
			t.Errorf("expected reason 'Dev simulation expired', got %v", dataMap["reason"])
		}
		if dataMap["status"] != "EXPIRED" {
			t.Errorf("expected status EXPIRED, got %v", dataMap["status"])
		}

		items, ok := dataMap["items"].([]interface{})
		if !ok || len(items) != 1 {
			t.Fatalf("expected 1 line item in event data, got %v", dataMap["items"])
		}
		firstItem := items[0].(map[string]interface{})
		if firstItem["productName"] != "Test Product" {
			t.Errorf("expected productName 'Test Product', got %v", firstItem["productName"])
		}
		if firstItem["quantity"].(float64) != 2 {
			t.Errorf("expected quantity 2, got %v", firstItem["quantity"])
		}
	})
}

