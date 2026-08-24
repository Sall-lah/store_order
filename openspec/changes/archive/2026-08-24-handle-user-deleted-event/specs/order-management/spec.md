## ADDED Requirements

### Requirement: Auto-Cancel Unpaid Orders on User Deletion
The system SHALL automatically transition all orders belonging to a deleted user that are in `PENDING_PAYMENT` status to `CANCELLED`, clearing payment tokens and persisting `order.cancelled` outbox events with reason `"User account deleted"` and full line items to notify downstream inventory services to release reserved stock.

#### Scenario: Unpaid orders auto-cancelled when user is deleted
- **WHEN** account deletion cleanup is executed for a user with one or more orders in `PENDING_PAYMENT` status
- **THEN** each unpaid order is updated to `CANCELLED`, active `snapToken` and `snapRedirectUrl` are cleared, and corresponding `order.cancelled` outbox events are created in a database transaction

#### Scenario: User has no unpaid orders
- **WHEN** account deletion cleanup is executed for a user with only `COMPLETED` or `CANCELLED` orders
- **THEN** order statuses remain unchanged and no cancellation outbox events are generated

### Requirement: Order PII Anonymization on User Deletion
The system SHALL redact and pseudonymize Personally Identifiable Information (PII) across all historical orders belonging to a deleted user, updating `userEmail` to an anonymized placeholder (e.g. `deleted_user_<hash>@anonymized.local`), setting `shippingAddress` to `[ANONYMIZED]`, and clearing temporary payment redirect credentials while strictly preserving order IDs, order numbers, line items, prices, financial totals, and payment gateway transaction IDs for accounting audit trails.

#### Scenario: Customer order PII anonymized
- **WHEN** account deletion cleanup is executed for a user with historical orders
- **THEN** the system updates all orders belonging to the `userId`, replacing `userEmail` with an anonymized pattern, setting `shippingAddress` to `[ANONYMIZED]`, and nullifying `snapToken` and `snapRedirectUrl` while keeping total amounts, line items, and payment references intact

#### Scenario: Querying anonymized orders in admin view
- **WHEN** an administrator views or exports historical orders containing anonymized accounts
- **THEN** the order details display the pseudonymized email and `[ANONYMIZED]` address without error while accurate sales totals and item quantities remain reflected in reports
