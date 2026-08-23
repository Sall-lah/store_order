## ADDED Requirements

### Requirement: Cancellation and Expiration Event Contracts
The system SHALL ensure that all `order.cancelled` and `order.expired` events persisted to the transactional outbox conform to the `store_notification` `OrderEventData` contract, containing the order identifier (`id`, `orderNumber`), customer email (`userEmail`), total amount (`totalAmount`), order status, line items (`items`), and contextual cancellation/expiration reason (`reason`).

#### Scenario: Order cancelled event envelope and payload validation
- **WHEN** an order transitions to `CANCELLED` status (via customer cancellation, admin update, Midtrans webhook, or dev simulation)
- **THEN** the system creates an outbox event on topic `order.events` with `event_type: "order.cancelled"` containing `orderNumber`, `userEmail`, `totalAmount`, `reason`, and line items

#### Scenario: Order expired event envelope and payload validation
- **WHEN** an order transitions to `EXPIRED` status (via Midtrans webhook or dev simulation)
- **THEN** the system creates an outbox event on topic `order.events` with `event_type: "order.expired"` containing `orderNumber`, `userEmail`, `totalAmount`, `reason`, and line items
