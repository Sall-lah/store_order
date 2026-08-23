## Why

Currently, `store_order` publishes sparse domain event payloads across fragmented Kafka topics with mismatched casing, missing critical attributes (such as active Snap payment redirect URLs, customer email, item lists, and delivery information). This prevents downstream services like `store_notification` from rendering customer email invoices and order status notifications. Additionally, fulfillment workflows lack fields to track the logistics courier name and receipt/tracking waybill number when transitioning orders to the `SHIPPED` state.

## What Changes

- **Snap Token Sequence in Checkout**: Generate Midtrans Snap payment tokens and redirect URLs *before* persisting order and outbox records, guaranteeing `snapRedirectUrl` is immediately captured in the `order.created` email invoice payload.
- **Consolidated Kafka Topic & Envelope**: Publish all order domain events to a single `order.events` Kafka topic partitioned by `order_id`, conforming to the `store_notification` OpenAPI contract (`event_id`, `event_type`, `timestamp`, `producer: "store_order"`, and rich `data`).
- **Complete Domain Event Triggers**:
  - `order.created`: Order placed email invoice with item breakdowns, pricing, and active `snapRedirectUrl`.
  - `order.paid`: Payment confirmation receipt email triggered by Midtrans settlement webhooks or dev payment simulation.
  - `order.shipped`: Shipping confirmation email triggered by admin fulfillment status updates, carrying courier name and receipt number.
  - `order.cancelled`: Cancellation notice email with cancellation reasons.
- **Database Model Enhancements**: Add nullable `courierName` and `receiptNumber` fields to the `Order` model in Prisma schema and database.
- **Admin Status Update Endpoint**: Accept optional `courierName` and `receiptNumber` in `PATCH /api/v1/admin/orders/{id}/status` when updating fulfillment status to `SHIPPED`.
- **API Documentation & OpenAPI Updates**: Update embedded OpenAPI 3.1 specifications (Scalar and Swagger UI) to document `courierName`, `receiptNumber`, and the new request/response schemas.

## Capabilities

### New Capabilities
<!-- No entirely new standalone capability needed; changes enhance existing order lifecycle and outbox streaming -->

### Modified Capabilities
- `order-management`: Enhanced checkout flow with upfront Snap token acquisition, addition of nullable `courierName` and `receiptNumber` columns to the Order model, administrative tracking metadata updates on status transition to `SHIPPED`, and rich domain event payload generation.
- `kafka-outbox`: Migration to consolidated `order.events` topic with standardized `EventEnvelope` (`event_id`, `event_type`, `timestamp`, `producer`, `data`) and `OrderEventData` payload contracts matching `store_notification`.

## Impact

- **Database**: Prisma schema updated with `courierName String?` and `receiptNumber String?` on `Order`; Prisma client regenerated.
- **Service Layer**: `internal/service/order_service.go` updated with full payload builder, upfront Snap token creation in `Checkout()`, and outbox event dispatch for `order.created`, `order.paid`, `order.shipped`, and `order.cancelled`.
- **Repository Layer**: `internal/repository/order_repository.go` and `repository/models.go` updated to persist `courierName` and `receiptNumber`.
- **Handler Layer**: `internal/handler/admin_handler.go` updated to parse `courierName` and `receiptNumber` in status updates.
- **Kafka**: Domain events published to `order.events` topic keyed by `order_id`.
- **Downstream**: `store_notification` can reliably consume from `order.events` to dispatch HTML email invoices for all 4 order milestones.
