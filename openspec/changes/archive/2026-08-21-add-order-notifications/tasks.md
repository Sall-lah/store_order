## 1. Database Schema & Models

- [x] 1.1 Add nullable `courierName` and `receiptNumber` fields to `Order` model in `prisma/schema.prisma`
- [x] 1.2 Regenerate Prisma Go client (`go run github.com/steebchen/prisma-client-go generate`) and update database schema

## 2. Event Envelope & Payload Standardization

- [x] 2.1 Update `DomainEventEnvelope` in `internal/service/order_service.go` to use `event_id`, `event_type`, `timestamp`, `producer: "store_order"`, and `data`
- [x] 2.2 Standardize topic constant to `TopicOrderEvents = "order.events"` and define event types (`order.created`, `order.paid`, `order.shipped`, `order.cancelled`)
- [x] 2.3 Implement centralized `buildOrderEventData` helper function in `internal/service/order_service.go` to build complete `OrderEventData` payloads

## 3. Data Layer & Repository Updates

- [x] 3.1 Add `CourierName` and `ReceiptNumber` fields to repository models and inputs in `internal/repository/models.go`
- [x] 3.2 Update `UpdateOrderStatusWithOutbox` in `internal/repository/order_repository.go` to persist `courierName` and `receiptNumber`

## 4. Service Layer Lifecycle & Notification Events

- [x] 4.1 Update `Checkout()` in `internal/service/order_service.go` to acquire Snap token upfront and capture `snapRedirectUrl` in `order.created` outbox event
- [x] 4.2 Update `ProcessMidtransWebhook` and `SimulatePaymentSuccess` to emit complete `order.paid` event payload
- [x] 4.3 Update `AdminUpdateStatus` to accept `courierName` & `receiptNumber` and emit `order.shipped` with tracking metadata
- [x] 4.4 Update `CancelCustomerOrder` and `SimulateOrderCancel` to emit full `order.cancelled` event payload
- [x] 4.5 Include `CourierName` and `ReceiptNumber` in `OrderResponse` and `ToOrderResponse`

## 5. HTTP Handler & API Layer Updates

- [x] 5.1 Update `AdminUpdateStatusRequest` in `internal/handler/admin_handler.go` to bind optional `courierName` and `receiptNumber`
- [x] 5.2 Update OpenAPI 3.1 YAML and JSON documentation under `docs/` with new fields and request/response schemas

## 6. Testing & Verification

- [x] 6.1 Update service and handler unit tests for checkout, cancellation, payment, and shipment status transitions
- [x] 6.2 Execute `go test ./...` to verify all test suites and compilation pass cleanly
