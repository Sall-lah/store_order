## Context

The `store_order` microservice manages order lifecycles and payment settlements. An independent downstream service (`store_notification`, documented at `http://localhost:8070/docs/notifications/openapi.json`) consumes Kafka events to render HTML email invoices and dispatch email notifications to customers for orders placed, paid, shipped, and cancelled.

Currently:
1. `store_order` produces outbox events to fragmented topics (`order.created`, `order.paid`, etc.) with non-conforming camelCase envelope tags (`eventId`, `eventType`), missing `producer`, and partial data.
2. In `Checkout()`, outbox events are inserted before requesting the Midtrans Snap token, resulting in missing `snapRedirectUrl` in the initial `order.created` email invoice.
3. The database `Order` model lacks fields to store the logistics courier name and receipt/waybill tracking number upon fulfillment.

## Goals / Non-Goals

**Goals:**
- Unify domain event publication under a single Kafka topic (`order.events`) partitioned by `order_id` for strict chronological ordering.
- Align event payloads with the `store_notification` OpenAPI schema: `EventEnvelope` (`event_id`, `event_type`, `timestamp`, `producer: "store_order"`, `data`) and complete `OrderEventData`.
- Acquire Midtrans Snap tokens upfront in `Checkout()` so `order.created` outbox events carry active payment URLs atomically.
- Support `order.created`, `order.paid`, `order.shipped`, and `order.cancelled` event triggers with full order and item details.
- Add nullable `courierName` and `receiptNumber` columns to the `Order` model in Prisma, supporting optional input during `PATCH /api/v1/admin/orders/{id}/status`.
- Update embedded Scalar and Swagger OpenAPI documentation to reflect new fields.

**Non-Goals:**
- Implementing SMTP transport or email template rendering in `store_order` (handled downstream by `store_notification`).
- Direct integration with third-party logistics tracking APIs (e.g. RajaOngkir/EasyParcel API lookups).

## Decisions

### Decision 1: Consolidated `order.events` Kafka Topic
- **Choice**: Publish all order events (`order.created`, `order.paid`, `order.shipped`, `order.cancelled`, `order.expired`) to `order.events` using `order.id` as the message key.
- **Rationale**: Guarantees Kafka partition-level sequential processing per order aggregate (e.g. `order.created` is guaranteed to be processed before `order.paid`) and matches the `store_notification` ingestion specification.
- **Alternatives Considered**: Separate topics per event type (rejected due to consumer partition racing and broker overhead).

### Decision 2: Upfront Snap Token Acquisition in Checkout
- **Choice**: Call Midtrans `CreateSnapTransaction` *before* opening the atomic database transaction that persists the `Order`, `OrderItem`s, and `OutboxEvent`.
- **Rationale**: Guarantees `snapRedirectUrl` is immediately captured in the `order.created` event payload for instant email invoice delivery with payment link, while preventing orphaned unpaid database records if Midtrans is unreachable.
- **Alternatives Considered**: Creating order first and updating outbox payload asynchronously (rejected due to race conditions and potential data inconsistency).

### Decision 3: Event Lifecycle Naming (`order.shipped`)
- **Choice**: Emit `order.shipped` when an administrator updates the order status to `SHIPPED`.
- **Rationale**: Clear domain semantics indicating that shipment is in progress and courier tracking information (`courierName`, `receiptNumber`) is available.

### Decision 4: Nullable Shipping Tracking Fields in Prisma Schema
- **Choice**: Add `courierName String?` and `receiptNumber String?` to the `Order` model.
- **Rationale**: Orders are created in `PENDING_PAYMENT` where courier tracking has not yet occurred. Admin provides these fields when transitioning fulfillment status to `SHIPPED`.

### Decision 5: Centralized `buildOrderEventData` Payload Builder
- **Choice**: Implement a shared helper in `internal/service/order_service.go` that maps `*db.OrderModel` and its line items into the standard `OrderEventData` map.
- **Rationale**: Enforces DRY payload construction across checkout, webhooks, cancellations, and status transitions, ensuring required attributes (`orderNumber`, `userEmail`, `totalAmount`, `items`) are never omitted.

## Risks / Trade-offs

- **[Risk] Midtrans API latency during checkout** → **Mitigation**: The Midtrans client is configured with bounded HTTP timeouts (5s); failures fail fast before touching the database.
- **[Risk] Database schema drift** → **Mitigation**: Update `prisma/schema.prisma` and regenerate Go client types with `prisma-client-go generate`.

## Migration Plan

1. Update `prisma/schema.prisma` with `courierName String?` and `receiptNumber String?`.
2. Push schema changes to database via Prisma and regenerate Go client code.
3. Update repository queries, DTOs, and service layer payload logic.
4. Run end-to-end integration and unit tests.
