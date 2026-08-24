## Context

The `store_order` service consumes asynchronous user lifecycle domain events over Apache Kafka (`user.events`). While account deletion (`user.deleted`) requires GDPR PII scrubbing and unpaid order cancellation, account suspension/banning (`user.banned`) requires a different balance: in-flight unpaid orders (`PENDING_PAYMENT`) must be cancelled to release reserved inventory back to the catalog, but all customer Personally Identifiable Information (PII) including `userEmail`, `shippingAddress`, and completed order histories must be retained completely intact for fraud prevention, chargeback defense, and accounting audit trails.

## Goals / Non-Goals

**Goals:**
- Extend `UserEventConsumer` in `internal/consumer/user_event_consumer.go` to parse and route `user.banned` events alongside `user.deleted`.
- Implement `OrderRepository.CancelUnpaidUserOrders` to atomically transition `PENDING_PAYMENT` orders to `CANCELLED`, clear payment tokens, and persist `order.cancelled` outbox events without touching customer email or shipping address.
- Implement `OrderService.HandleUserBanned` to orchestrate validation, order discovery, outbox construction, and repository updates.
- Ensure strict PII preservation across all historical orders belonging to the banned account.
- Guarantee at-least-once message delivery idempotency.

**Non-Goals:**
- Anonymizing or modifying `userEmail` or `shippingAddress` for banned accounts (strictly prohibited for audit retention).
- Cancelling or refunding `PAID`, `PROCESSING`, `SHIPPED`, or `COMPLETED` orders during ban event processing.
- Modifying authentication or API Gateway ban enforcement (handled upstream by `store_gateway` and `store_auth`).

## Decisions

### 1. Dedicated Repository Method (`CancelUnpaidUserOrders`)
- **Choice**: Implement a dedicated `CancelUnpaidUserOrders(ctx context.Context, userID string, outboxEvents []OutboxCreateInput) error` method on `OrderRepository` rather than parameterizing `AnonymizeUserOrdersAndCancelUnpaid`.
- **Rationale**: Strict architectural separation of concerns eliminates the possibility of accidental PII masking or regression during account ban operations.
- **Alternatives Considered**: Adding a boolean `anonymizePII` flag to `AnonymizeUserOrdersAndCancelUnpaid` (rejected: increases coupling and risk of configuration error causing unintended data loss).

### 2. Multi-Event Extraction & Routing in Consumer
- **Choice**: Upgrade `ExtractUserEvent` in `internal/consumer` to return `(userID string, eventType string, err error)` supporting both `user.deleted` and `user.banned` event types.
- **Rationale**: Allows the single `store_order_user_events` Kafka consumer group to process all user lifecycle events on `user.events` sequentially and dispatch to appropriate domain handlers.
- **Alternatives Considered**: Separate Kafka consumer groups for each event type (rejected: creates redundant connection overhead on the same topic).

### 3. Outbox Event Generation for Stock Restock
- **Choice**: Emit `order.cancelled` outbox records for each cancelled `PENDING_PAYMENT` order with reason `"User account banned"`.
- **Rationale**: Reuses the Transactional Outbox pattern so downstream inventory services (`store_product`) release stock reservations asynchronously and reliably.

## Risks / Trade-offs

- **[Risk] Duplicate Kafka message delivery** → *Mitigation*: Service queries only `PENDING_PAYMENT` orders for `userID`. If orders are already cancelled, zero outbox events are created and repository update is a safe no-op.
- **[Risk] Banned user has completed orders** → *Mitigation*: Only `PENDING_PAYMENT` orders are transitioned to `CANCELLED`; `PAID`, `SHIPPED`, and `COMPLETED` orders remain untouched with full historical PII.
