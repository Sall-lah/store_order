## 1. Domain Event Payloads & Verification

- [x] 1.1 Verify and standardize `buildOrderEventData` in `internal/service/order_service.go` ensuring all required fields (`orderNumber`, `userEmail`, `totalAmount`, `reason`, `items`) are populated for `order.cancelled` and `order.expired` events
- [x] 1.2 Verify customer cancellation, admin status update, and dev cancellation handlers in `internal/service/order_service.go` emit compliant `order.cancelled` outbox events
- [x] 1.3 Verify Midtrans webhook processor and dev simulation in `internal/service/order_service.go` emit compliant `order.expired` and `order.cancelled` outbox events

## 2. Unit Testing & Schema Validation

- [x] 2.1 Add test cases in `internal/service/order_service_test.go` to assert `order.cancelled` outbox event envelope and payload schema conformity
- [x] 2.2 Add test cases in `internal/service/order_service_test.go` to assert `order.expired` outbox event envelope and payload schema conformity
- [x] 2.3 Run full test suite with `go test ./...` to ensure zero regressions

## 3. Documentation & OpenAPI Updates

- [x] 3.1 Update `docs/openapi.json` and `docs/openapi.yaml` to document Kafka event schemas for `order.cancelled` and `order.expired`
- [x] 3.2 Run format checks and verify all artifacts
