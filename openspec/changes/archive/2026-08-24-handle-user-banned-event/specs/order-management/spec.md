## ADDED Requirements

### Requirement: Auto-Cancel Unpaid Orders on User Ban
The system SHALL automatically transition all orders belonging to a banned user that are in `PENDING_PAYMENT` status to `CANCELLED`, clear active payment tokens (`snapToken`, `snapRedirectUrl`), and persist `order.cancelled` outbox events with reason `"User account banned"` and full line items to trigger reserved stock release in downstream inventory services.

#### Scenario: Unpaid orders auto-cancelled when user is banned
- **WHEN** ban cleanup is executed for a user with one or more orders in `PENDING_PAYMENT` status
- **THEN** each unpaid order is updated to `CANCELLED`, payment redirect credentials are cleared, and corresponding `order.cancelled` outbox events are created in a database transaction

#### Scenario: User has no unpaid orders when banned
- **WHEN** ban cleanup is executed for a user who has only `PAID`, `PROCESSING`, `SHIPPED`, `COMPLETED`, or already `CANCELLED` orders
- **THEN** order statuses remain unchanged and no cancellation outbox events are generated

### Requirement: PII Preservation on User Ban
The system SHALL strictly preserve all customer Personally Identifiable Information (PII), including `userEmail`, `shippingAddress`, order notes, total amounts, line items, and payment references without redaction across all orders belonging to a banned user.

#### Scenario: Customer order PII preserved upon account ban
- **WHEN** ban cleanup is executed for a user with historical orders
- **THEN** `userEmail` and `shippingAddress` remain untouched with original customer values to preserve evidence for fraud investigation and chargeback dispute defense

#### Scenario: Admin views orders for banned user
- **WHEN** an administrator views historical orders belonging to a banned user
- **THEN** the system displays the original customer email and full shipping address alongside order details
