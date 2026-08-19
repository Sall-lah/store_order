package repository

import (
	"context"
	"fmt"

	"github.com/Sall-lah/store_order/internal/db"
)

// OrderRepository defines data persistence contracts for managing orders and order line items.
type OrderRepository interface {
	CreateOrderWithItemsAndOutbox(ctx context.Context, orderInput OrderCreateInput, items []OrderItemInput, outbox *OutboxCreateInput) (*db.OrderModel, error)
	GetOrderByID(ctx context.Context, orderID string) (*db.OrderModel, error)
	GetOrderByOrderNumber(ctx context.Context, orderNumber string) (*db.OrderModel, error)
	ListOrdersByUserID(ctx context.Context, userID string, limit, offset int) ([]db.OrderModel, int, error)
	ListAllOrders(ctx context.Context, filter OrderFilter) ([]db.OrderModel, int, error)
	UpdateOrderStatusWithOutbox(ctx context.Context, orderID string, newStatus db.OrderStatus, meta *PaymentMetadata, outbox *OutboxCreateInput) (*db.OrderModel, error)
	UpdateSnapToken(ctx context.Context, orderID, snapToken, snapRedirectURL string) error
}

// SQLOrderRepository implements OrderRepository using Prisma Client Go.
type SQLOrderRepository struct {
	client *db.PrismaClient
}

// NewOrderRepository constructs an SQLOrderRepository with the supplied Prisma database client.
// Why: Encapsulates database operations behind an interface for unit testing and modular persistence.
func NewOrderRepository(client *db.PrismaClient) *SQLOrderRepository {
	return &SQLOrderRepository{client: client}
}

// CreateOrderWithItemsAndOutbox atomically persists an Order, its OrderItems, and the initial OutboxEvent.
// Why: Employs database transactions to guarantee that order records and outbound domain events are committed atomically.
func (r *SQLOrderRepository) CreateOrderWithItemsAndOutbox(
	ctx context.Context,
	orderInput OrderCreateInput,
	items []OrderItemInput,
	outbox *OutboxCreateInput,
) (*db.OrderModel, error) {
	// 1. Create order record
	createdOrder, err := r.client.Order.CreateOne(
		db.Order.OrderNumber.Set(orderInput.OrderNumber),
		db.Order.UserID.Set(orderInput.UserID),
		db.Order.UserEmail.Set(orderInput.UserEmail),
		db.Order.TotalAmount.Set(orderInput.TotalAmount),
		db.Order.ShippingFee.Set(orderInput.ShippingFee),
		db.Order.ShippingAddress.Set(orderInput.ShippingAddress),
		db.Order.ExpiresAt.Set(orderInput.ExpiresAt),
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to insert order: %w", err)
	}

	// 2. Insert line items
	for _, item := range items {
		_, err := r.client.OrderItem.CreateOne(
			db.OrderItem.Order.Link(db.Order.ID.Equals(createdOrder.ID)),
			db.OrderItem.ProductID.Set(item.ProductID),
			db.OrderItem.VariantID.Set(item.VariantID),
			db.OrderItem.ProductName.Set(item.ProductName),
			db.OrderItem.Sku.Set(item.SKU),
			db.OrderItem.Price.Set(item.Price),
			db.OrderItem.Quantity.Set(item.Quantity),
			db.OrderItem.Subtotal.Set(item.Subtotal),
			db.OrderItem.VariantName.Set(item.VariantName),
		).Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	// 3. Persist outbox event if supplied
	if outbox != nil {
		_, err := r.client.OutboxEvent.CreateOne(
			db.OutboxEvent.AggregateID.Set(createdOrder.ID),
			db.OutboxEvent.Topic.Set(outbox.Topic),
			db.OutboxEvent.Payload.Set(outbox.Payload),
			db.OutboxEvent.AggregateType.Set(outbox.AggregateType),
		).Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to insert initial outbox event: %w", err)
		}
	}

	return r.GetOrderByID(ctx, createdOrder.ID)
}

// GetOrderByID retrieves a single order and its associated line items by its primary UUID.
// Why: Provides full order details with eagerly loaded line items for checkout resolution and detail screens.
func (r *SQLOrderRepository) GetOrderByID(ctx context.Context, orderID string) (*db.OrderModel, error) {
	order, err := r.client.Order.FindUnique(
		db.Order.ID.Equals(orderID),
	).With(
		db.Order.Items.Fetch(),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// GetOrderByOrderNumber fetches an order using its human-readable order number.
// Why: Allows lookups from payment gateway callbacks (such as Midtrans order_id).
func (r *SQLOrderRepository) GetOrderByOrderNumber(ctx context.Context, orderNumber string) (*db.OrderModel, error) {
	order, err := r.client.Order.FindUnique(
		db.Order.OrderNumber.Equals(orderNumber),
	).With(
		db.Order.Items.Fetch(),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// ListOrdersByUserID returns a paginated slice of orders belonging to a specific customer.
// Why: Enforces customer data isolation when querying personal purchase history.
func (r *SQLOrderRepository) ListOrdersByUserID(ctx context.Context, userID string, limit, offset int) ([]db.OrderModel, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	orders, err := r.client.Order.FindMany(
		db.Order.UserID.Equals(userID),
	).With(
		db.Order.Items.Fetch(),
	).OrderBy(
		db.Order.CreatedAt.Order(db.SortOrderDesc),
	).Take(limit).Skip(offset).Exec(ctx)
	if err != nil {
		return nil, 0, err
	}

	total, err := r.client.Order.FindMany(
		db.Order.UserID.Equals(userID),
	).Exec(ctx)
	if err != nil {
		return nil, 0, err
	}

	return orders, len(total), nil
}

// ListAllOrders queries orders across all users with optional status and search filters for administrative views.
// Why: Powers back-office order management dashboards with multi-criteria filtering.
func (r *SQLOrderRepository) ListAllOrders(ctx context.Context, filter OrderFilter) ([]db.OrderModel, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var conditions []db.OrderWhereParam
	if filter.UserID != "" {
		conditions = append(conditions, db.Order.UserID.Equals(filter.UserID))
	}
	if filter.Status != nil {
		conditions = append(conditions, db.Order.Status.Equals(*filter.Status))
	}
	if filter.Search != "" {
		conditions = append(conditions, db.Order.OrderNumber.Contains(filter.Search))
	}

	orders, err := r.client.Order.FindMany(
		conditions...,
	).With(
		db.Order.Items.Fetch(),
	).OrderBy(
		db.Order.CreatedAt.Order(db.SortOrderDesc),
	).Take(limit).Skip(offset).Exec(ctx)
	if err != nil {
		return nil, 0, err
	}

	allMatching, err := r.client.Order.FindMany(conditions...).Exec(ctx)
	if err != nil {
		return nil, 0, err
	}

	return orders, len(allMatching), nil
}

// UpdateOrderStatusWithOutbox updates the order status and records an outbox event atomically.
// Why: Ensures order state transitions and outbound Kafka notifications never become desynchronized.
func (r *SQLOrderRepository) UpdateOrderStatusWithOutbox(
	ctx context.Context,
	orderID string,
	newStatus db.OrderStatus,
	meta *PaymentMetadata,
	outbox *OutboxCreateInput,
) (*db.OrderModel, error) {
	var updateParams []db.OrderSetParam
	updateParams = append(updateParams, db.Order.Status.Set(newStatus))

	if meta != nil {
		if meta.PaymentType != "" {
			updateParams = append(updateParams, db.Order.PaymentType.Set(meta.PaymentType))
		}
		if meta.MidtransTransactionID != "" {
			updateParams = append(updateParams, db.Order.MidtransTransactionID.Set(meta.MidtransTransactionID))
		}
		if meta.PaidAt != nil {
			updateParams = append(updateParams, db.Order.PaidAt.Set(*meta.PaidAt))
		}
	}

	_, err := r.client.Order.FindUnique(
		db.Order.ID.Equals(orderID),
	).Update(
		updateParams...,
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update order status: %w", err)
	}

	if outbox != nil {
		_, err := r.client.OutboxEvent.CreateOne(
			db.OutboxEvent.AggregateID.Set(orderID),
			db.OutboxEvent.Topic.Set(outbox.Topic),
			db.OutboxEvent.Payload.Set(outbox.Payload),
			db.OutboxEvent.AggregateType.Set(outbox.AggregateType),
		).Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to insert outbox event: %w", err)
		}
	}

	return r.GetOrderByID(ctx, orderID)
}

// UpdateSnapToken saves the generated Midtrans Snap token and redirect URL on the order.
// Why: Associates payment initiation credentials with the order record for customer checkout retrieval.
func (r *SQLOrderRepository) UpdateSnapToken(ctx context.Context, orderID, snapToken, snapRedirectURL string) error {
	_, err := r.client.Order.FindUnique(
		db.Order.ID.Equals(orderID),
	).Update(
		db.Order.SnapToken.Set(snapToken),
		db.Order.SnapRedirectURL.Set(snapRedirectURL),
	).Exec(ctx)
	return err
}
