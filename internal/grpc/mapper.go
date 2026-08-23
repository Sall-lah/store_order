package grpc

import (
	"time"

	"github.com/Sall-lah/store_order/internal/db"
	orderv1 "github.com/Sall-lah/store_proto/gen/go/store/order/v1"
)

// MapDBStatusToProto converts a database OrderStatus enum into the corresponding Protobuf OrderStatus enum.
// Why: Bridges internal Prisma persistence domain models to the standardized inter-service gRPC contract.
func MapDBStatusToProto(dbStatus db.OrderStatus) orderv1.OrderStatus {
	switch dbStatus {
	case db.OrderStatusPendingPayment:
		return orderv1.OrderStatus_ORDER_STATUS_PENDING_PAYMENT
	case db.OrderStatusPaid:
		return orderv1.OrderStatus_ORDER_STATUS_PAID
	case db.OrderStatusProcessing:
		return orderv1.OrderStatus_ORDER_STATUS_PROCESSING
	case db.OrderStatusShipped:
		return orderv1.OrderStatus_ORDER_STATUS_SHIPPED
	case db.OrderStatusCompleted:
		return orderv1.OrderStatus_ORDER_STATUS_COMPLETED
	case db.OrderStatusCancelled:
		return orderv1.OrderStatus_ORDER_STATUS_CANCELLED
	case db.OrderStatusExpired:
		return orderv1.OrderStatus_ORDER_STATUS_EXPIRED
	default:
		return orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

// MapProtoStatusToDB converts a Protobuf OrderStatus enum back into a database OrderStatus enum.
// Why: Provides bidirectional domain mapping when receiving status-based gRPC filters or mutations.
func MapProtoStatusToDB(protoStatus orderv1.OrderStatus) (db.OrderStatus, bool) {
	switch protoStatus {
	case orderv1.OrderStatus_ORDER_STATUS_PENDING_PAYMENT:
		return db.OrderStatusPendingPayment, true
	case orderv1.OrderStatus_ORDER_STATUS_PAID:
		return db.OrderStatusPaid, true
	case orderv1.OrderStatus_ORDER_STATUS_PROCESSING:
		return db.OrderStatusProcessing, true
	case orderv1.OrderStatus_ORDER_STATUS_SHIPPED:
		return db.OrderStatusShipped, true
	case orderv1.OrderStatus_ORDER_STATUS_COMPLETED:
		return db.OrderStatusCompleted, true
	case orderv1.OrderStatus_ORDER_STATUS_CANCELLED:
		return db.OrderStatusCancelled, true
	case orderv1.OrderStatus_ORDER_STATUS_EXPIRED:
		return db.OrderStatusExpired, true
	default:
		return "", false
	}
}

// MapOrderModelToActiveSummary transforms an active database OrderModel into a lightweight gRPC ActiveOrderSummary.
// Why: Emits only essential fields (ID, reference number, status, amount, timestamp) to optimize gRPC wire payload size.
func MapOrderModelToActiveSummary(order db.OrderModel) *orderv1.ActiveOrderSummary {
	return &orderv1.ActiveOrderSummary{
		OrderId:     order.ID,
		OrderNumber: order.OrderNumber,
		Status:      MapDBStatusToProto(order.Status),
		TotalAmount: order.TotalAmount,
		CreatedAt:   order.CreatedAt.Format(time.RFC3339),
	}
}
