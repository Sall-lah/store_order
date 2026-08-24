## Why

When a user deletes their account in the authentication service (`store_user`), downstream commerce records in `store_order` must comply with data privacy regulations (such as GDPR/Right to be Forgotten) without corrupting financial, accounting, tax, or inventory reconciliation ledgers. Additionally, any unpaid in-flight orders (`PENDING_PAYMENT`) must be automatically cancelled to release reserved inventory back to the catalog.

## What Changes

- **Kafka Consumer Worker**: Introduce a background Kafka consumer group worker listening to user lifecycle events (specifically `user.deleted` on `user.events` topic).
- **Auto-Cancellation of Unpaid Orders**: Automatically transition all `PENDING_PAYMENT` orders belonging to the deleted user to `CANCELLED` and emit `order.cancelled` outbox events with reason `"User account deleted"` to trigger stock release in `store_product`.
- **PII Anonymization**: Pseudonymize and redact all Personally Identifiable Information (PII) across all historical orders belonging to the deleted user (`userEmail` masked, `shippingAddress` redacted, `snapToken`/`snapRedirectUrl` cleared) while preserving essential financial transaction data (`totalAmount`, `paymentType`, `midtransTransactionId`, `items`).
- **Idempotency & Resilience**: Ensure the consumer processes `user.deleted` events idempotently so duplicate Kafka message deliveries do not cause invalid state transitions or duplicate side effects.

## Capabilities

### New Capabilities
- `user-lifecycle-events`: Handles asynchronous user lifecycle events from Kafka (e.g. `user.deleted`), orchestrating order cancellation for unpaid orders and initiating PII scrubbing.

### Modified Capabilities
- `order-management`: Adds repository and service capabilities to execute PII anonymization and batch cancellation for deleted customer accounts.

## Impact

- **Database**: Updates `Order` records (`userEmail`, `shippingAddress`, `snapToken`, `snapRedirectUrl`, `status`) and inserts `OutboxEvent` records for cancelled orders.
- **Kafka**: Introduces a consumer worker subscribed to the user events topic (`user.events`) alongside the existing producer and outbox publisher.
- **Microservices**: Emits `order.cancelled` events on `order.events` to notify inventory services of released stock.
- **Runtime**: Boots Kafka consumer group gracefully within `cmd/server/main.go` with signal handling for clean shutdown.
