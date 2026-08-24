## Context

The `store_order` service manages checkout, payments, and order tracking. It already provides a synchronous gRPC pre-flight check (`CheckActiveOrders`) to prevent users with active orders from deleting their accounts. However, once an account deletion is approved and finalized in the auth service (`store_user`), the auth service asynchronously emits a `user.deleted` event on Kafka (`user.events`).

`store_order` must subscribe to this event and perform two critical operations:
1. Auto-cancel any lingering unpaid orders (`PENDING_PAYMENT`) and emit `order.cancelled` outbox events to release inventory reservations in `store_product`.
2. Anonymize/redact all PII (`userEmail`, `shippingAddress`, Snap tokens) across all historical orders belonging to that customer while preserving financial totals, tax records, payment IDs, and line items for accounting and tax audits.

## Goals / Non-Goals

**Goals:**
- Implement a Kafka consumer group worker (`UserEventConsumer`) in `internal/consumer` subscribed to `user.events`.
- Implement `OrderRepository.AnonymizeUserOrdersAndCancelUnpaid` to atomically cancel unpaid orders, insert `order.cancelled` outbox records, and scrub PII across all matching orders.
- Implement `OrderService.HandleUserDeleted` to orchestrate validation, execution, and logging.
- Ensure idempotent execution for at-least-once message delivery guarantees.
- Integrate consumer lifecycle (start and graceful shutdown) into `cmd/server/main.go`.

**Non-Goals:**
- Hard deleting `Order` or `OrderItem` rows (violates tax, accounting, and financial auditability standards).
- Handling active `PAID` or `SHIPPED` order refunding during event consumption (these are guarded upfront by gRPC pre-flight check).

## Decisions

### 1. Dedicated Consumer Group Worker
- **Choice**: Create `internal/consumer/user_event_consumer.go` using `segmentio/kafka-go.NewReader` with `GroupID: "store_order_user_events"`.
- **Rationale**: `segmentio/kafka-go` is already imported in the project. Consumer groups allow horizontal scaling and automatic partition offset tracking across multiple order service replicas.
- **Alternatives Considered**: Direct HTTP webhook from auth service (rejected: creates synchronous coupling and lacks retry durability on order service downtime).

### 2. Transactional Cancellation and PII Scrubbing
- **Choice**: Execute unpaid order cancellation, outbox event generation, and PII anonymization within a single atomic database operation.
- **Rationale**: Prevents partial failure states where orders are anonymized but unpaid orders remain uncancelled (leaving inventory permanently reserved), or vice-versa.
- **Anonymization Schema**:
  - `userEmail` -> `deleted_user_<hash_or_id>@anonymized.local`
  - `shippingAddress` -> `"[ANONYMIZED]"`
  - `snapToken` -> `""` / null
  - `snapRedirectUrl` -> `""` / null

### 3. Outbox Integration for Stock Release
- **Choice**: For every cancelled `PENDING_PAYMENT` order, insert an `OutboxEvent` for topic `order.events` with event type `order.cancelled` and reason `"User account deleted"`.
- **Rationale**: Reuses the established Outbox pattern in `store_order`. The background outbox publisher delivers the event to notify `store_product` / inventory service without introducing synchronous RPCs inside the consumer loop.

### 4. Concurrency & Graceful Shutdown
- **Choice**: Boot `UserEventConsumer.Start(ctx)` in a separate goroutine in `cmd/server/main.go`, and cancel its context / call `Close()` on `SIGINT` / `SIGTERM` alongside the HTTP, gRPC, and Outbox workers.
- **Rationale**: Ensures zero dropped messages and graceful completion of in-flight batch operations on container termination.

## Risks / Trade-offs

- **[Risk] At-least-once Kafka duplicate delivery** → *Mitigation*: The repository query finds orders by `userId`. Anonymizing already anonymized fields is idempotent, and non-pending orders are skipped during the cancel step.
- **[Risk] High order volume per user slows down transaction** → *Mitigation*: The batch update operates on indexed `userId`. Typical consumer account order counts are well within normal relational transaction boundaries.
- **[Risk] Consumer lag during service maintenance** → *Mitigation*: Kafka retains messages during downtime; when the service boots, the consumer group resumes from the committed offset.
