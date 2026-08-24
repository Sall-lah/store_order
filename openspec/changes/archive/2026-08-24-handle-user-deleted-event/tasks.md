## 1. Repository & Data Layer

- [x] 1.1 Add `AnonymizeUserOrdersAndCancelUnpaid` method to `OrderRepository` interface and implementation in `internal/repository/order_repository.go`
- [x] 1.2 Add unit tests for repository order anonymization and cancellation in `internal/repository/order_repository_test.go`

## 2. Order Service Domain Logic

- [x] 2.1 Add `HandleUserDeleted(ctx context.Context, userID string) error` to `OrderService` interface and implementation in `internal/service/order_service.go`
- [x] 2.2 Add unit tests for `HandleUserDeleted` in `internal/service/order_service_test.go` covering unpaid order cancellation, outbox event generation, and PII anonymization

## 3. Kafka Consumer Implementation

- [x] 3.1 Create `UserEventConsumer` in `internal/consumer/user_event_consumer.go` using `segmentio/kafka-go`
- [x] 3.2 Add unit tests for `UserEventConsumer` message deserialization and dispatch in `internal/consumer/user_event_consumer_test.go`

## 4. Application Integration & Lifecycle

- [x] 4.1 Update `internal/config/config.go` with user events Kafka topic and consumer group settings
- [x] 4.2 Wire and start `UserEventConsumer` in `cmd/server/main.go` alongside HTTP, gRPC, and Outbox workers with graceful shutdown
- [x] 4.3 Run full test suite (`go test ./...`) to verify all modules and integration flows
