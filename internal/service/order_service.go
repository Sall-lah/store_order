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
	TopicOrderEvents = "order.events"

	// Legacy topic aliases retained for backwards compatibility
	TopicOrderCreated   = "order.created"
	TopicOrderPaid      = "order.paid"
	TopicOrderCancelled = "order.cancelled"
	TopicOrderExpired   = "order.expired"
	TopicOrderFulfilled = "order.fulfilled"
	TopicOrderShipped   = "order.shipped"
)

// Domain event types
const (
	EventTypeOrderCreated   = "order.created"
	EventTypeOrderPaid      = "order.paid"
	EventTypeOrderShipped   = "order.fulfilled"
	EventTypeOrderCancelled = "order.cancelled"
	EventTypeOrderExpired   = "order.expired"
)

// DomainEventEnvelope wraps domain events with standardized metadata for downstream consumer dispatch.
type DomainEventEnvelope struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"`
	Timestamp string      `json:"timestamp"`
	Producer  string      `json:"producer"`
	Data      interface{} `json:"data"`
}

// NewDomainEventEnvelope constructs an EventEnvelope with default producer and UTC timestamp.
// Why: Standardizes event envelope serialization conforming to the platform notification specification.
func NewDomainEventEnvelope(eventType string, data interface{}) DomainEventEnvelope {
	return DomainEventEnvelope{
		EventID:   uuid.NewString(),
		EventType: eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Producer:  "store_order",
		Data:      data,
	}
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
	CourierName           string              `json:"courierName,omitempty"`
	ReceiptNumber         string              `json:"receiptNumber,omitempty"`
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
	AdminUpdateStatus(ctx context.Context, orderID string, newStatus string, courierName string, receiptNumber string) (*OrderResponse, error)
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

// Checkout validates item prices, persists order + outbox records atomically, and acquires Midtrans Snap tokens upfront.
// Why: Provides a unified, atomic checkout transaction guaranteeing price integrity, payment readiness, and complete event invoice payloads.
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

	var repoItems []repository.OrderItemInput
	var snapItemDetails []midtrans.ItemDetail
	var eventItems []map[string]interface{}

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

		itemMap := map[string]interface{}{
			"id":          vi.ProductID,
			"productId":   vi.ProductID,
			"product_id":  vi.ProductID,
			"variantId":   vi.VariantID,
			"variant_id":  vi.VariantID,
			"productName": vi.ProductName,
			"sku":         vi.SKU,
			"price":       vi.UnitPrice,
			"quantity":    vi.Quantity,
			"subtotal":    vi.Subtotal,
		}
		if vi.VariantName != "" {
			itemMap["variantName"] = vi.VariantName
			itemMap["variant_name"] = vi.VariantName
		}
		eventItems = append(eventItems, itemMap)
	}

	if req.ShippingFee > 0 {
		snapItemDetails = append(snapItemDetails, midtrans.ItemDetail{
			ID:       "SHIPPING",
			Price:    req.ShippingFee,
			Quantity: 1,
			Name:     "Shipping Fee",
		})
	}

	// 2. Acquire Midtrans Snap token upfront
	var snapToken, snapRedirectURL string
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
	if err == nil && snapResp != nil {
		snapToken = snapResp.Token
		snapRedirectURL = snapResp.RedirectURL
	}

	// 3. Prepare repo order input with Snap token
	orderInput := repository.OrderCreateInput{
		OrderNumber:     orderNumber,
		UserID:          userID,
		UserEmail:       userEmail,
		TotalAmount:     totalAmount,
		ShippingFee:     req.ShippingFee,
		ShippingAddress: req.ShippingAddress,
		SnapToken:       snapToken,
		SnapRedirectURL: snapRedirectURL,
		ExpiresAt:       expiresAt,
	}

	// 4. Prepare initial order.created outbox event payload with active Snap payment redirect URL
	eventData := map[string]interface{}{
		"id":                orderNumber,
		"order_id":          orderNumber,
		"orderNumber":       orderNumber,
		"order_number":      orderNumber,
		"userId":            userID,
		"user_id":           userID,
		"userEmail":         userEmail,
		"user_email":        userEmail,
		"status":            string(db.OrderStatusPendingPayment),
		"totalAmount":       totalAmount,
		"total_amount":      totalAmount,
		"shippingFee":       req.ShippingFee,
		"shipping_fee":      req.ShippingFee,
		"shippingAddress":   req.ShippingAddress,
		"shipping_address":  req.ShippingAddress,
		"snapRedirectUrl":   snapRedirectURL,
		"snap_redirect_url": snapRedirectURL,
		"createdAt":         time.Now().UTC().Format(time.RFC3339),
		"created_at":        time.Now().UTC().Format(time.RFC3339),
		"expiresAt":         expiresAt.UTC().Format(time.RFC3339),
		"expires_at":        expiresAt.UTC().Format(time.RFC3339),
		"items":             eventItems,
	}

	eventPayloadBytes, _ := json.Marshal(NewDomainEventEnvelope(EventTypeOrderCreated, eventData))

	outboxInput := &repository.OutboxCreateInput{
		AggregateType: "ORDER",
		Topic:         TopicOrderEvents,
		Payload:       string(eventPayloadBytes),
	}

	// 5. Atomically persist order, line items, and outbox event
	createdOrder, err := s.orderRepo.CreateOrderWithItemsAndOutbox(ctx, orderInput, repoItems, outboxInput)
	if err != nil {
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	return ToOrderResponse(createdOrder), nil
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

	order.Status = db.OrderStatusCancelled
	eventPayload, _ := json.Marshal(NewDomainEventEnvelope(EventTypeOrderCancelled, buildOrderEventData(order, "Customer cancelled")))

	outbox := &repository.OutboxCreateInput{
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		Topic:         TopicOrderEvents,
		Payload:       string(eventPayload),
	}

	updated, err := s.orderRepo.UpdateOrderStatusWithOutbox(ctx, orderID, db.OrderStatusCancelled, nil, nil, outbox)
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

	var eventType string
	var reason string
	switch targetStatus {
	case db.OrderStatusPaid:
		eventType = EventTypeOrderPaid
	case db.OrderStatusCancelled:
		eventType = EventTypeOrderCancelled
		reason = "Midtrans transaction cancelled or denied"
	case db.OrderStatusExpired:
		eventType = EventTypeOrderExpired
		reason = "Payment expired"
	}

	now := time.Now()
	meta := &repository.PaymentMetadata{
		PaymentType:           notif.PaymentType,
		MidtransTransactionID: notif.TransactionID,
		PaidAt:                &now,
	}

	var outbox *repository.OutboxCreateInput
	if eventType != "" {
		order.Status = targetStatus
		eventData := buildOrderEventData(order, reason)
		eventData["paymentType"] = notif.PaymentType
		eventData["midtransTransactionId"] = notif.TransactionID
		if targetStatus == db.OrderStatusPaid {
			eventData["paidAt"] = now.UTC().Format(time.RFC3339)
		}

		eventPayload, _ := json.Marshal(NewDomainEventEnvelope(eventType, eventData))
		outbox = &repository.OutboxCreateInput{
			AggregateType: "ORDER",
			AggregateID:   order.ID,
			Topic:         TopicOrderEvents,
			Payload:       string(eventPayload),
		}
	}

	_, err = s.orderRepo.UpdateOrderStatusWithOutbox(ctx, order.ID, targetStatus, meta, nil, outbox)
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
// Why: Allows store administrators to update fulfillment workflow, record logistics tracking, and dispatch downstream notifications.
func (s *Service) AdminUpdateStatus(ctx context.Context, orderID string, newStatusStr string, courierName string, receiptNumber string) (*OrderResponse, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, errors.New("order not found")
	}

	var targetStatus db.OrderStatus
	var eventType string
	switch strings.ToUpper(strings.TrimSpace(newStatusStr)) {
	case "PROCESSING":
		targetStatus = db.OrderStatusProcessing
	case "SHIPPED":
		targetStatus = db.OrderStatusShipped
		eventType = EventTypeOrderShipped
	case "COMPLETED":
		targetStatus = db.OrderStatusCompleted
	case "CANCELLED":
		targetStatus = db.OrderStatusCancelled
		eventType = EventTypeOrderCancelled
	default:
		return nil, fmt.Errorf("invalid target order status: %s", newStatusStr)
	}

	var shipping *repository.ShippingMetadata
	if courierName != "" || receiptNumber != "" {
		shipping = &repository.ShippingMetadata{
			CourierName:   courierName,
			ReceiptNumber: receiptNumber,
		}
	}

	var outbox *repository.OutboxCreateInput
	if eventType != "" {
		order.Status = targetStatus
		eventData := buildOrderEventData(order, "")
		if courierName != "" {
			eventData["courierName"] = courierName
		}
		if receiptNumber != "" {
			eventData["receiptNumber"] = receiptNumber
		}
		if targetStatus == db.OrderStatusCancelled {
			eventData["reason"] = "Admin cancelled"
		}

		eventPayload, _ := json.Marshal(NewDomainEventEnvelope(eventType, eventData))
		outbox = &repository.OutboxCreateInput{
			AggregateType: "ORDER",
			AggregateID:   order.ID,
			Topic:         TopicOrderEvents,
			Payload:       string(eventPayload),
		}
	}

	updated, err := s.orderRepo.UpdateOrderStatusWithOutbox(ctx, orderID, targetStatus, nil, shipping, outbox)
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

	order.Status = db.OrderStatusPaid
	eventData := buildOrderEventData(order, "")
	eventData["paymentType"] = meta.PaymentType
	eventData["midtransTransactionId"] = meta.MidtransTransactionID
	eventData["paidAt"] = now.UTC().Format(time.RFC3339)

	eventPayload, _ := json.Marshal(NewDomainEventEnvelope(EventTypeOrderPaid, eventData))
	outbox := &repository.OutboxCreateInput{
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		Topic:         TopicOrderEvents,
		Payload:       string(eventPayload),
	}

	updated, err := s.orderRepo.UpdateOrderStatusWithOutbox(ctx, orderID, db.OrderStatusPaid, meta, nil, outbox)
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

	order.Status = db.OrderStatusExpired
	eventPayload, _ := json.Marshal(NewDomainEventEnvelope(EventTypeOrderExpired, buildOrderEventData(order, "Dev simulation expired")))

	outbox := &repository.OutboxCreateInput{
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		Topic:         TopicOrderEvents,
		Payload:       string(eventPayload),
	}

	updated, err := s.orderRepo.UpdateOrderStatusWithOutbox(ctx, orderID, db.OrderStatusExpired, nil, nil, outbox)
	if err != nil {
		return nil, err
	}

	return ToOrderResponse(updated), nil
}

// buildOrderEventData extracts full order metadata and line items into standard OrderEventData schema.
// Why: Provides a single source of truth for constructing domain notification payloads across all lifecycle events conforming to the store_notification OpenAPI contract.
func buildOrderEventData(order *db.OrderModel, reason string) map[string]interface{} {
	if order == nil {
		return map[string]interface{}{}
	}

	items := make([]map[string]interface{}, 0)
	if order.RelationsOrder.Items != nil {
		for _, item := range order.RelationsOrder.Items {
			itemMap := map[string]interface{}{
				"id":          item.ID,
				"productId":   item.ProductID,
				"product_id":  item.ProductID,
				"variantId":   item.VariantID,
				"variant_id":  item.VariantID,
				"productName": item.ProductName,
				"sku":         item.Sku,
				"price":       item.Price,
				"quantity":    item.Quantity,
				"subtotal":    item.Subtotal,
			}
			if vName, ok := item.VariantName(); ok && vName != "" {
				itemMap["variantName"] = vName
				itemMap["variant_name"] = vName
			}
			items = append(items, itemMap)
		}
	}

	data := map[string]interface{}{
		"id":           order.ID,
		"order_id":     order.ID,
		"orderNumber":  order.OrderNumber,
		"order_number": order.OrderNumber,
		"userId":       order.UserID,
		"user_id":      order.UserID,
		"userEmail":    order.UserEmail,
		"user_email":   order.UserEmail,
		"status":       string(order.Status),
		"totalAmount":  order.TotalAmount,
		"total_amount": order.TotalAmount,
		"shippingFee":  order.ShippingFee,
		"shipping_fee": order.ShippingFee,
		"createdAt":    order.CreatedAt.UTC().Format(time.RFC3339),
		"created_at":   order.CreatedAt.UTC().Format(time.RFC3339),
		"items":        items,
	}

	if addr, ok := order.ShippingAddress(); ok && addr != "" {
		data["shippingAddress"] = addr
	}
	if pType, ok := order.PaymentType(); ok && pType != "" {
		data["paymentType"] = pType
	}
	if snapURL, ok := order.SnapRedirectURL(); ok && snapURL != "" {
		data["snapRedirectUrl"] = snapURL
	}
	if cName, ok := order.CourierName(); ok && cName != "" {
		data["courierName"] = cName
	}
	if rNum, ok := order.ReceiptNumber(); ok && rNum != "" {
		data["receiptNumber"] = rNum
	}
	if paidAt, ok := order.PaidAt(); ok {
		data["paidAt"] = paidAt.UTC().Format(time.RFC3339)
	}
	if reason != "" {
		data["reason"] = reason
	}

	return data
}

// ToOrderResponse maps a database OrderModel to a client DTO.
// Why: Encapsulates database mapping and formats timestamps and relations consistently.
func ToOrderResponse(m *db.OrderModel) *OrderResponse {
	if m == nil {
		return nil
	}

	resp := &OrderResponse{
		ID:          m.ID,
		OrderNumber: m.OrderNumber,
		UserID:      m.UserID,
		UserEmail:   m.UserEmail,
		Status:      string(m.Status),
		TotalAmount: m.TotalAmount,
		ShippingFee: m.ShippingFee,
		CreatedAt:   m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   m.UpdatedAt.Format(time.RFC3339),
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
	if cName, ok := m.CourierName(); ok {
		resp.CourierName = cName
	}
	if rNum, ok := m.ReceiptNumber(); ok {
		resp.ReceiptNumber = rNum
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
