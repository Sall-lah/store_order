## Why

When an administrator bans or suspends a fraudulent or abusive customer account in `store_user`, in-flight unpaid orders (`PENDING_PAYMENT`) must be automatically cancelled to release reserved inventory back to the catalog. Unlike user account deletion (which redacts PII for GDPR compliance), user account banning requires full retention of customer Personally Identifiable Information (PII), shipping addresses, and transaction audit trails to defend against chargeback disputes, assist fraud investigations, and preserve legal evidence.

## What Changes

- **Kafka Event Consumer Support**: Extend `UserEventConsumer` listening on topic `user.events` to parse and handle `user.banned` domain events containing `user_id` and optional `reason`.
- **Auto-Cancellation of Unpaid Orders**: Automatically transition all `PENDING_PAYMENT` orders belonging to the banned user to `CANCELLED` and clear active payment tokens.
- **Stock Release Outbox Events**: Persist `order.cancelled` outbox events with reason `"User account banned"` and full line item details on topic `order.events` to notify `store_product` to release reserved inventory.
- **Audit & PII Preservation**: Strictly preserve all historical orders, completed transactions, customer emails (`userEmail`), and shipping addresses (`shippingAddress`) without redaction or pseudonymization.
- **Idempotency & Resilience**: Ensure duplicate or replay deliveries of `user.banned` execute as safe no-ops without duplicate cancellation events.

## Capabilities

### Modified Capabilities

- `order-management`: Adds repository and service capabilities to execute unpaid order cancellation for banned users while strictly preserving customer PII.
- `user-lifecycle-events`: Expands Kafka event consumption contract to handle `user.banned` domain events alongside `user.deleted`.

## Impact

- **Database**: Transitions status to `CANCELLED` and clears payment redirect tokens for `PENDING_PAYMENT` orders belonging to the banned user; inserts `OutboxEvent` records for inventory release.
- **Kafka**: Expands consumer message routing on `user.events` topic and emits `order.cancelled` events to `order.events`.
- **Services**: `store_product` receives `order.cancelled` events and restocks catalog inventory.
