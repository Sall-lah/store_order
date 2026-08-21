package repository

import (
	"time"

	"github.com/Sall-lah/store_order/internal/db"
)

// OrderItemInput represents line item details submitted during checkout creation.
type OrderItemInput struct {
	ProductID   string
	VariantID   string
	ProductName string
	VariantName string
	SKU         string
	Price       float64
	Quantity    int
	Subtotal    float64
}

// OrderCreateInput holds attributes required to persist a new order.
type OrderCreateInput struct {
	OrderNumber     string
	UserID          string
	UserEmail       string
	TotalAmount     float64
	ShippingFee     float64
	ShippingAddress string
	SnapToken       string
	SnapRedirectURL string
	CourierName     string
	ReceiptNumber   string
	ExpiresAt       time.Time
}

// OutboxCreateInput contains payload and metadata for an atomic outbox event.
type OutboxCreateInput struct {
	AggregateType string
	AggregateID   string
	Topic         string
	Payload       string
}

// PaymentMetadata contains payment confirmation attributes from Midtrans or Dev simulator.
type PaymentMetadata struct {
	PaymentType           string
	MidtransTransactionID string
	PaidAt                *time.Time
}

// ShippingMetadata contains logistics shipment tracking attributes provided by administrators.
type ShippingMetadata struct {
	CourierName   string
	ReceiptNumber string
}

// OrderFilter options for querying orders.
type OrderFilter struct {
	UserID    string
	Status    *db.OrderStatus
	Search    string
	Limit     int
	Offset    int
	SortOrder string
}
