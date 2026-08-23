## Why

The `store_notification` microservice has been updated to consume and deliver automated customer email notifications for order expiration (`order.expired`) and cancellation (`order.cancelled`). To guarantee reliable notification delivery and downstream inventory coordination, `store_order` must align its event payload contracts with the notification service OpenAPI specification, ensuring that all lifecycle triggers consistently emit complete order metadata, line items, and contextual reason descriptions.

## What Changes

- Align `order.cancelled` and `order.expired` transactional outbox event payloads with the `store_notification` OpenAPI specification (`OrderEventData`).
- Ensure all cancellation pathways (Customer cancellation, Admin cancellation, Midtrans cancel/deny webhook, Dev simulation) consistently populate the `reason`, `items`, `orderNumber`, `userEmail`, and `totalAmount` attributes.
- Ensure all expiration pathways (Midtrans expire webhook, Dev simulation) consistently populate the `reason`, `items`, `orderNumber`, `userEmail`, and `totalAmount` attributes.
- Expand unit test suites in `order_service_test.go` to validate schema conformity of `order.cancelled` and `order.expired` outbox payloads across all lifecycle state transitions.
- Update `store_order` OpenAPI documentation (`docs/openapi.json` and `docs/openapi.yaml`) to document Kafka event schemas for `order.cancelled` and `order.expired`.

## Capabilities

### New Capabilities
<!-- None -->

### Modified Capabilities
- `kafka-outbox`: Explicitly define requirements and payload contracts for `order.cancelled` and `order.expired` domain events on topic `order.events`.
- `order-management`: Ensure order cancellation and expiration state transitions capture complete line items and reason descriptions in outbox events.

## Impact

- **Services**: `store_order`, `store_notification`, downstream inventory listeners (`store_product`).
- **Code**: `internal/service/order_service.go`, `internal/service/order_service_test.go`.
- **Documentation**: `docs/openapi.json`, `docs/openapi.yaml`.
