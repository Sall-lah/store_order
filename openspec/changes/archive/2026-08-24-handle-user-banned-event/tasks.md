## 1. Repository Layer

- [x] 1.1 Add `CancelUnpaidUserOrders(ctx context.Context, userID string, outboxEvents []OutboxCreateInput) error` to `OrderRepository` interface and implementation in `internal/repository/order_repository.go`
- [x] 1.2 Add unit tests for `CancelUnpaidUserOrders` in `internal/repository/order_repository_test.go` verifying PII preservation, unpaid order cancellation, and outbox persistence

## 2. Order Service Domain Logic

- [x] 2.1 Add `HandleUserBanned(ctx context.Context, userID string, reason string) error` to `OrderService` interface and implementation in `internal/service/order_service.go`
- [x] 2.2 Add unit tests for `HandleUserBanned` in `internal/service/order_service_test.go` verifying unpaid order cancellation, `order.cancelled` outbox event generation, and strict customer PII retention

## 3. Kafka Consumer & Event Routing

- [x] 3.1 Update `UserEventConsumer` in `internal/consumer/user_event_consumer.go` to parse and route `user.banned` events to `HandleUserBanned`
- [x] 3.2 Add unit tests in `internal/consumer/user_event_consumer_test.go` for `user.banned` event extraction, domain dispatch, and error handling

## 4. Verification & Testing

- [x] 4.1 Run full test suite (`go test ./...`) and build server binary (`go build ./cmd/server`) to verify all modules and integration flows
