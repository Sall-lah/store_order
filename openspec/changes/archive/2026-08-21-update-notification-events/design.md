## Context

The `store_order` service publishes order lifecycle events to Apache Kafka (`order.events`) using a Transactional Outbox pattern. Downstream, `store_notification` consumes these events to dispatch transactional emails to customers and administrators.

With `store_notification` now providing automated email handlers for `order.cancelled` (`HandleOrderCancelled`) and `order.expired` (`HandleOrderExpired`), `store_order` must ensure strict contract compliance with the `OrderEventData` schema defined in `store_notification`'s OpenAPI specification.

## Goals / Non-Goals

**Goals:**
- Guarantee that all `order.cancelled` events emitted by `store_order` (customer cancellation, admin status update, Midtrans cancel/deny webhook, dev simulation) contain complete `OrderEventData` payloads with `orderNumber`, `userEmail`, `totalAmount`, `reason`, and line `items`.
- Guarantee that all `order.expired` events emitted by `store_order` (Midtrans expire webhook, dev simulation) contain complete `OrderEventData` payloads with `orderNumber`, `userEmail`, `totalAmount`, `reason`, and line `items`.
- Maintain single-source-of-truth event construction via `buildOrderEventData`.
- Provide automated unit tests verifying envelope formatting and payload schema conformity.
- Update `store_order` OpenAPI schemas and documentation to describe `order.cancelled` and `order.expired` events.

**Non-Goals:**
- Building an automated background order expiration scheduler/sweeper (expiration continues to be triggered via Midtrans webhooks and dev simulations).
- Changing database schema or Kafka topic names (`order.events` remains the primary topic).

## Decisions

### Decision 1: Single Source of Truth for Outbox Payloads (`buildOrderEventData`)
- **Rationale**: `buildOrderEventData` extracts order identifiers, user email, totals, shipping information, payment metadata, line items, and contextual reason strings into the standard payload format expected by `store_notification`.
- **Alternative Considered**: Constructing bespoke ad-hoc maps in each handler. Rejected due to drift risk and missing fields.

### Decision 2: Contextual Cancellation and Expiration Reasons
- **Rationale**: The `reason` field in `OrderEventData` is rendered directly in customer-facing notification templates (e.g., "[Store Platform] Order #ORD-xxx Cancelled"). Providing clear, descriptive strings improves user clarity.
- **Values**:
  - Customer cancel: `"Customer cancelled"`
  - Admin cancel: `"Admin cancelled"`
  - Midtrans cancel/deny: `"Midtrans transaction cancelled or denied"`
  - Midtrans expire: `"Payment expired"`
  - Dev simulation: `"Dev simulation expired"` / `"Customer cancelled"`

### Decision 3: Contract Verification in Unit Tests
- **Rationale**: Add test cases in `order_service_test.go` that inspect the JSON payloads of `outboxEvents` created during customer cancellation, webhook expiration/cancellation, admin cancellation, and dev simulations to prevent schema regression.

## Risks / Trade-offs

- **[Risk]** Incomplete item details if order entity is loaded without item relations.
  - **Mitigation**: Verify that `GetOrderByID` and `GetOrderByOrderNumber` populate item relations or ensure `buildOrderEventData` handles partial structures safely while tests validate relation loading.
