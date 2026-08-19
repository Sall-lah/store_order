package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Sall-lah/store_order/internal/db"
	"github.com/Sall-lah/store_order/internal/integration/midtrans"
	"github.com/Sall-lah/store_order/internal/integration/product"
	"github.com/Sall-lah/store_order/internal/repository"
)

// Kafka event topics
const (
	TopicOrderCreated   = "order.created"
	TopicOrderPaid      = "order.paid"
	TopicOrderCancelled = "order.cancelled"
	TopicOrderExpired   = "order.expired"
	TopicOrderFulfilled = "order.fulfilled"
)

// DomainEventEnvelope wraps domain events with metadata for downstream consumer dispatch.
type DomainEventEnvelope struct {
	EventID   string      `json:"eventId"`
	EventType string      `json:"eventType"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// OrderItemResponse formats line item details for client responses.
type OrderItemResponse struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"productId"`
	VariantID   string  `json:"variantId"`
	ProductName string  `json:"productName"`
	VariantName string  `json:"variantName,omitempty"`
	SKU         string  `json:"sku"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
	Subtotal    float64 `json:"subtotal"`
}

// OrderResponse represents full order details for client serialization.
type OrderResponse struct {
	ID                    string              `json:"id"`
	OrderNumber           string              `json:"orderNumber"`
	UserID                string              `json:"userId"`
	UserEmail             string              `json:"userEmail"`
	Status                string              `json:"status"`
	TotalAmount           float64             `json:"totalAmount"`
	ShippingFee           float64             `json:"shippingFee"`
	ShippingAddress       string              `json:"shippingAddress,omitempty"`
	PaymentType           string              `json:"paymentType,omitempty"`
	MidtransTransactionID string              `json:"midtransTransactionId,omitempty"`
	SnapToken             string              `json:"snapToken,omitempty"`
	SnapRedirectURL       string              `json:"snapRedirectUrl,omitempty"`
	PaidAt                *string             `json:"paidAt,omitempty"`
	ExpiresAt             *string             `json:"expiresAt,omitempty"`
	Items                 []OrderItemResponse `json:"items,omitempty"`
	CreatedAt             string              `json:"createdAt"`
	UpdatedAt             string              `json:"updatedAt"`
}

// OrderListResponse represents a paginated list of orders.
type OrderListResponse struct {
	Orders []OrderResponse `json:"orders"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// CheckoutRequest defines customer order payload.
type CheckoutRequest struct {
	Items           []product.ItemOrderRequest `json:"items"`
	ShippingFee     float64                    `json:"shippingFee"`
	ShippingAddress string                     `json:"shippingAddress"`
}

// OrderService defines high-level business capabilities for the order domain.
type OrderService interface {
	Checkout(ctx context.Context, userID, userEmail string, req CheckoutRequest) (*OrderResponse, error)
	GetCustomerOrder(ctx context.Context, userID, orderID string) (*OrderResponse, error)
	ListCustomerOrders(ctx context.Context, userID string, limit, offset int) (*OrderListResponse, error)
	CancelCustomerOrder(ctx context.Context, userID, orderID string) (*OrderResponse, error)
	ProcessMidtransWebhook(ctx context.Context, notif midtrans.WebhookNotification) error
	AdminListOrders(ctx context.Context, filter repository.OrderFilter) (*OrderListResponse, error)
	AdminUpdateStatus(ctx context.Context, orderID string, newStatus string) (*OrderResponse, error)
	SimulatePaymentSuccess(ctx context.Context, orderID string) (*OrderResponse, error)
	SimulateOrderCancel(ctx context.Context, orderID string) (*OrderResponse, error)
	SimulateOrderExpire(ctx context.Context, orderID string) (*OrderResponse, error)
}

// Service coordinates order business logic, external integrations, and atomic outbox updates.
type Service struct {
	orderRepo      repository.OrderRepository
	productClient  product.Client
	midtransClient midtrans.Client
	serverKey      string
	isDevMode      bool
}

// NewOrderService constructs an OrderService instance with injected dependencies.
// Why: Enables clean dependency inversion, mocking for test isolation, and decoupled domain logic.
func NewOrderService(
	orderRepo repository.OrderRepository,
	productClient product.Client,
	midtransClient midtrans.Client,
	serverKey string,
	isDevMode bool,
) *Service {
	return &Service{
		orderRepo:      orderRepo,
		productClient:  productClient,
		midtransClient: midtransClient,
		serverKey:      serverKey,
		isDevMode:      isDevMode,
	}
}

// Checkout validates item prices, persists order + outbox records atomically, and acquires Midtrans Snap tokens.
// Why: Provides a unified, atomic checkout transaction guaranteeing price integrity and payment readiness.
func (s *Service) Checkout(ctx context.Context, userID, userEmail string, req CheckoutRequest) (*OrderResponse, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("authenticated user ID is required")
	}

	// 1. Authoritative price verification with product service
	validatedItems, itemsSubtotal, err := s.productClient.ValidateItems(ctx, req.Items)
	if err != nil {
		return nil, fmt.Errorf("item validation failed: %w", err)
	}

	totalAmount := itemsSubtotal + req.ShippingFee
	orderNumber := generateOrderNumber()
	expiresAt := time.Now().Add(24 * time.Hour)

	// 2. Prepare repo inputs
	orderInput := repository.OrderCreateInput{
		OrderNumber:     orderNumber,
		UserID:          userID,
		UserEmail:       userEmail,
		TotalAmount:     totalAmount,
		ShippingFee:     req.ShippingFee,
		ShippingAddress: req.ShippingAddress,
		ExpiresAt:       expiresAt,
	}

	var repoItems []repository.OrderItemInput
	var snapItemDetails []midtrans.ItemDetail

	for _, vi := range validatedItems {
		repoItems = append(repoItems, repository.OrderItemInput{
			ProductID:   vi.ProductID,
			VariantID:   vi.VariantID,
			ProductName: vi.ProductName,
			VariantName: vi.VariantName,
			SKU:         vi.SKU,
			Price:       vi.UnitPrice,
			Quantity:    vi.Quantity,
			Subtotal:    vi.Subtotal,
		})

		snapItemDetails = append(snapItemDetails, midtrans.ItemDetail{
			ID:       vi.SKU,
			Price:    vi.UnitPrice,
			Quantity: vi.Quantity,
			Name:     vi.ProductName,
		})
	}

	if req.ShippingFee > 0 {
		snapItemDetails = append(snapItemDetails, midtrans.ItemDetail{
			ID:       "SHIPPING",
			Price:    req.ShippingFee,
			Quantity: 1,
			Name:     "Shipping Fee",
		})
	}

	// 3. Prepare initial order.created outbox event payload
	eventPayloadBytes, _ := json.Marshal(DomainEventEnvelope{
		EventID:   uuid.NewString(),
		EventType: TopicOrderCreated,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"orderNumber": orderNumber,
			"userId":      userID,
			"userEmail":   userEmail,
			"totalAmount": totalAmount,
			"expiresAt":   expiresAt.Format(time.RFC3339),
			"items":       validatedItems,
		},
	})

	outboxInput := &repository.OutboxCreateInput{
		AggregateType: "ORDER",
		Topic:         TopicOrderCreated,
		Payload:       string(eventPayloadBytes),
	}

	// 4. Atomically persist order, line items, and outbox event
	createdOrder, err := s.orderRepo.CreateOrderWithItemsAndOutbox(ctx, orderInput, repoItems, outboxInput)
	if err != nil {
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	orderResp := ToOrderResponse(createdOrder)

	// 5. Generate Midtrans Snap Token
	snapReq := midtrans.SnapTransactionRequest{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:     orderNumber,
			GrossAmount: totalAmount,
		},
		CustomerDetails: midtrans.CustomerDetails{
			FirstName: userEmail,
			Email:     userEmail,
		},
		ItemDetails: snapItemDetails,
	}

	snapResp, err := s.midtransClient.CreateSnapTransaction(ctx, snapReq)
	if err != nil {
		return orderResp, nil
	}

	_ = s.orderRepo.UpdateSnapToken(ctx, createdOrder.ID, snapResp.Token, snapResp.RedirectURL)
	orderResp.SnapToken = snapResp.Token
	orderResp.SnapRedirectURL = snapResp.RedirectURL

	return orderResp, nil
}

// GetCustomerOrder retrieves a specific order after validating ownership.
// Why: Guarantees customer data isolation and prevents unauthorized order inspection.
func (s *Service) GetCustomerOrder(ctx context.Context, userID, orderID string) (*OrderResponse, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.New("order not found")
	}

	if order.UserID != userID {
		return nil, errors.New("forbidden: you do not own this order")
	}

	return ToOrderResponse(order), nil
}

// ListCustomerOrders retrieves a customer's personal order history.
// Why: Powers the customer account order history view.
func (s *Service) ListCustomerOrders(ctx context.Context, userID string, limit, offset int) (*OrderListResponse, error) {
	orders, total, err := s.orderRepo.ListOrdersByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	var respOrders []OrderResponse
	for _, o := range orders {
		respOrders = append(respOrders, *ToOrderResponse(&o))
	}

	return &OrderListResponse{
		Orders: respOrders,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// CancelCustomerOrder allows customers to cancel their order if it is pending payment.
// Why: Lets users abort unpaid orders and emits order.cancelled to release any reservations.
func (s *Service) CancelCustomerOrder(ctx context.Context, userID, orderID string) (*OrderResponse, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.New("order not found")
	}

	if userID != "" && order.UserID != userID {
		return nil, errors.New("forbidden: you do not own this order")
	}

	if order.Status != db.OrderStatusPendingPayment {
		return nil, fmt.Errorf("cannot cancel order in %s status", order.Status)
	}

	eventPayload, _ := json.Marshal(DomainEventEnvelope{
		EventID:   uuid.NewString(),
		EventType: TopicOrderCancelled,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"orderId":     order.ID,
			"orderNumber": order.OrderNumber,
			"userId":      order.UserID,
			"reason":      "Customer cancelled",
		},
	})

	outbox := &repository.OutboxCreateInput{
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		Topic:         TopicOrderCancelled,
		Payload:       string(eventPayload),
	}

	updated, err := s.orderRepo.UpdateOrderStatusWithOutbox(ctx, orderID, db.OrderStatusCancelled, nil, outbox)
	if err != nil {
		return nil, err
	}

	return ToOrderResponse(updated), nil
}

// ProcessMidtransWebhook validates signature and updates order status with atomic Kafka outbox event.
// Why: Processes asynchronous payment settlement, cancellation, or expiration from Midtrans securely and idempotently.
func (s *Service) ProcessMidtransWebhook(ctx context.Context, notif midtrans.WebhookNotification) error {
	// Verify SHA-512 cryptographic signature
	if !s.isDevMode && !midtrans.VerifySignature(s.serverKey, notif) {
		return errors.New("invalid midtrans webhook signature")
	}

	order, err := s.orderRepo.GetOrderByOrderNumber(ctx, notif.OrderID)
	if err != nil || order == nil {
		return fmt.Errorf("order not found for order number %s: %w", notif.OrderID, err)
	}

	targetStatus, ok := midtrans.DetermineOrderStatus(notif)
	if !ok {
		return nil // Ignore unknown transient status
	}

	// Idempotency check: if order is already in target status, acknowledge without duplicate processing
	if order.Status == targetStatus {
		return nil
	}

	var topic string
	switch targetStatus {
	case db.OrderStatusPaid:
		topic = TopicOrderPaid
	case db.OrderStatusCancelled:
		topic = TopicOrderCancelled
	case db.OrderStatusExpired:
		topic = TopicOrderExpired
	default:
		topic = ""
	}

	now := time.Now()
	meta := &repository.PaymentMetadata{
		PaymentType:           notif.PaymentType,
		MidtransTransactionID: notif.TransactionID,
		PaidAt:                &now,
	}

	var outbox *repository.OutboxCreateInput
	if topic != "" {
		eventPayload, _ := json.Marshal(DomainEventEnvelope{
			EventID:   uuid.NewString(),
			EventType: topic,
			Timestamp: now.UTC().Format(time.RFC3339),
			Data: map[string]interface{}{
				"orderId":               order.ID,
				"orderNumber":           order.OrderNumber,
				"userId":                order.UserID,
				"userEmail":             order.UserEmail,
				"totalAmount":           order.TotalAmount,
				"paymentType":           notif.PaymentType,
				"midtransTransactionId": notif.TransactionID,
			},
		})
		outbox = &repository.OutboxCreateInput{
			AggregateType: "ORDER",
			AggregateID:   order.ID,
			Topic:         topic,
			Payload:       string(eventPayload),
		}
	}

	_, err = s.orderRepo.UpdateOrderStatusWithOutbox(ctx, order.ID, targetStatus, meta, outbox)
	return err
}

// AdminListOrders queries orders system-wide with filtering.
// Why: Powers admin dashboard order management.
func (s *Service) AdminListOrders(ctx context.Context, filter repository.OrderFilter) (*OrderListResponse, error) {
	orders, total, err := s.orderRepo.ListAllOrders(ctx, filter)
	if err != nil {
		return nil, err
	}

	var respOrders []OrderResponse
	for _, o := range orders {
		respOrders = append(respOrders, *ToOrderResponse(&o))
	}

	return &OrderListResponse{
		Orders: respOrders,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

// AdminUpdateStatus transitions fulfillment state (e.g. PROCESSING, SHIPPED, COMPLETED).
// Why: Allows store administrators to update fulfillment workflow and dispatch downstream notifications.
func (s *Service) AdminUpdateStatus(ctx context.Context, orderID string, newStatusStr string) (*OrderResponse, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.New("order not found")
	}

	var targetStatus db.OrderStatus
	switch strings.ToUpper(strings.TrimSpace(newStatusStr)) {
	case "PROCESSING":
		targetStatus = db.OrderStatusProcessing
	case "SHIPPED":
		targetStatus = db.OrderStatusShipped
	case "COMPLETED":
		targetStatus = db.OrderStatusCompleted
	case "CANCELLED":
		targetStatus = db.OrderStatusCancelled
	default:
		return nil, fmt.Errorf("invalid target order status: %s", newStatusStr)
	}

	var outbox *repository.OutboxCreateInput
	if targetStatus == db.OrderStatusShipped || targetStatus == db.OrderStatusCompleted {
		eventPayload, _ := json.Marshal(DomainEventEnvelope{
			EventID:   uuid.NewString(),
			EventType: TopicOrderFulfilled,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Data: map[string]interface{}{
				"orderId":     order.ID,
				"orderNumber": order.OrderNumber,
				"status":      string(targetStatus),
			},
		})
		outbox = &repository.OutboxCreateInput{
			AggregateType: "ORDER",
			AggregateID:   order.ID,
			Topic:         TopicOrderFulfilled,
			Payload:       string(eventPayload),
		}
	}

	updated, err := s.orderRepo.UpdateOrderStatusWithOutbox(ctx, orderID, targetStatus, nil, outbox)
	if err != nil {
		return nil, err
	}

	return ToOrderResponse(updated), nil
}

// SimulatePaymentSuccess simulates a successful settlement callback in DEV mode.
// Why: Allows rapid offline manual and automated testing of payment flow and Kafka publication.
func (s *Service) SimulatePaymentSuccess(ctx context.Context, orderID string) (*OrderResponse, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.New("order not found")
	}

	now := time.Now()
	meta := &repository.PaymentMetadata{
		PaymentType:           "dev_simulation_qris",
		MidtransTransactionID: fmt.Sprintf("sim_tx_%s", uuid.NewString()[:8]),
		PaidAt:                &now,
	}

	eventPayload, _ := json.Marshal(DomainEventEnvelope{
		EventID:   uuid.NewString(),
		EventType: TopicOrderPaid,
		Timestamp: now.UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"orderId":               order.ID,
			"orderNumber":           order.OrderNumber,
			"userId":                order.UserID,
			"userEmail":             order.UserEmail,
			"totalAmount":           order.TotalAmount,
			"paymentType":           meta.PaymentType,
			"midtransTransactionId": meta.MidtransTransactionID,
		},
	})

	outbox := &repository.OutboxCreateInput{
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		Topic:         TopicOrderPaid,
		Payload:       string(eventPayload),
	}

	updated, err := s.orderRepo.UpdateOrderStatusWithOutbox(ctx, orderID, db.OrderStatusPaid, meta, outbox)
	if err != nil {
		return nil, err
	}

	return ToOrderResponse(updated), nil
}

// SimulateOrderCancel simulates cancellation in DEV mode.
// Why: Facilitates testing stock release events without waiting for real payment expiry.
func (s *Service) SimulateOrderCancel(ctx context.Context, orderID string) (*OrderResponse, error) {
	return s.CancelCustomerOrder(ctx, "", orderID)
}

// SimulateOrderExpire simulates expiration in DEV mode.
// Why: Tests order expiry and inventory release event dispatch.
func (s *Service) SimulateOrderExpire(ctx context.Context, orderID string) (*OrderResponse, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.New("order not found")
	}

	eventPayload, _ := json.Marshal(DomainEventEnvelope{
		EventID:   uuid.NewString(),
		EventType: TopicOrderExpired,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"orderId":     order.ID,
			"orderNumber": order.OrderNumber,
			"userId":      order.UserID,
		},
	})

	outbox := &repository.OutboxCreateInput{
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		Topic:         TopicOrderExpired,
		Payload:       string(eventPayload),
	}

	updated, err := s.orderRepo.UpdateOrderStatusWithOutbox(ctx, orderID, db.OrderStatusExpired, nil, outbox)
	if err != nil {
		return nil, err
	}

	return ToOrderResponse(updated), nil
}

// ToOrderResponse maps a database OrderModel to a client DTO.
// Why: Encapsulates database mapping and formats timestamps and relations consistently.
func ToOrderResponse(m *db.OrderModel) *OrderResponse {
	if m == nil {
		return nil
	}

	resp := &OrderResponse{
		ID:              m.ID,
		OrderNumber:     m.OrderNumber,
		UserID:          m.UserID,
		UserEmail:       m.UserEmail,
		Status:          string(m.Status),
		TotalAmount:     m.TotalAmount,
		ShippingFee:     m.ShippingFee,
		CreatedAt:       m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       m.UpdatedAt.Format(time.RFC3339),
	}

	if addr, ok := m.ShippingAddress(); ok {
		resp.ShippingAddress = addr
	}
	if ptype, ok := m.PaymentType(); ok {
		resp.PaymentType = ptype
	}
	if txID, ok := m.MidtransTransactionID(); ok {
		resp.MidtransTransactionID = txID
	}
	if token, ok := m.SnapToken(); ok {
		resp.SnapToken = token
	}
	if url, ok := m.SnapRedirectURL(); ok {
		resp.SnapRedirectURL = url
	}
	if paidAt, ok := m.PaidAt(); ok {
		str := paidAt.Format(time.RFC3339)
		resp.PaidAt = &str
	}
	if expAt, ok := m.ExpiresAt(); ok {
		str := expAt.Format(time.RFC3339)
		resp.ExpiresAt = &str
	}

	if m.RelationsOrder.Items != nil {
		for _, item := range m.RelationsOrder.Items {
			itemResp := OrderItemResponse{
				ID:          item.ID,
				ProductID:   item.ProductID,
				VariantID:   item.VariantID,
				ProductName: item.ProductName,
				SKU:         item.Sku,
				Price:       item.Price,
				Quantity:    item.Quantity,
				Subtotal:    item.Subtotal,
			}
			if vName, ok := item.VariantName(); ok {
				itemResp.VariantName = vName
			}
			resp.Items = append(resp.Items, itemResp)
		}
	}

	return resp
}

func generateOrderNumber() string {
	now := time.Now().Format("20060102")
	randomSuffix := strings.ToUpper(uuid.NewString()[:6])
	return fmt.Sprintf("ORD-%s-%s", now, randomSuffix)
}
