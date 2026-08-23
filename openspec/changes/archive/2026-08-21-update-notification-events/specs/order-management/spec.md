## MODIFIED Requirements

### Requirement: Customer Order Cancellation
The system SHALL permit customers to cancel their own orders if the order is still in `PENDING_PAYMENT` status, persisting an `order.cancelled` outbox event with complete order metadata and reason.

#### Scenario: Customer cancels unpaid order
- **WHEN** a customer sends `POST /api/v1/orders/{id}/cancel` for an order in `PENDING_PAYMENT` status owned by them
- **THEN** the system transitions the order status to `CANCELLED`, persists an outbox event for `order.cancelled` with order number, customer email, total amount, reason (`"Customer cancelled"`), and line items, and returns HTTP 200

#### Scenario: Customer attempts to cancel already paid order
- **WHEN** a customer sends `POST /api/v1/orders/{id}/cancel` for an order with status `PAID` or `SHIPPED`
- **THEN** the system rejects the cancellation with HTTP 400 Bad Request stating the order cannot be cancelled in its current state

## ADDED Requirements

### Requirement: Order Expiration and Webhook Cancellation Handling
The system SHALL process asynchronous Midtrans expiration and cancellation webhooks as well as dev simulations by transitioning the order status and emitting compliant `order.expired` or `order.cancelled` outbox events with explicit reason descriptions and line items for downstream notification dispatch.

#### Scenario: Midtrans expire webhook updates order and dispatches notification event
- **WHEN** a valid Midtrans webhook with `transaction_status: "expire"` is received for an active order
- **THEN** the system updates the order status to `EXPIRED` and writes an `order.expired` outbox event with reason `"Payment expired"` and full line items

#### Scenario: Midtrans cancel or deny webhook updates order and dispatches notification event
- **WHEN** a valid Midtrans webhook with `transaction_status: "cancel"` or `"deny"` is received for an active order
- **THEN** the system updates the order status to `CANCELLED` and writes an `order.cancelled` outbox event with reason `"Midtrans transaction cancelled or denied"` and full line items
