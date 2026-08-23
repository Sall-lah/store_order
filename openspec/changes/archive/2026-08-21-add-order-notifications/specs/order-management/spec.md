## ADDED Requirements

### Requirement: Order Shipping Tracking Metadata
The system SHALL support nullable `courierName` and `receiptNumber` attributes on the Order entity, permitting tracking waybill and logistics courier recording during the fulfillment lifecycle.

#### Scenario: Nullable tracking fields on checkout
- **WHEN** an order is created during customer checkout
- **THEN** `courierName` and `receiptNumber` are stored as null and returned as null/omitted in client order responses

#### Scenario: Tracking fields updated upon shipment
- **WHEN** an administrator updates an order's status to `SHIPPED` providing `courierName` and `receiptNumber`
- **THEN** the values are persisted to the database and exposed in subsequent order detail and list queries

## MODIFIED Requirements

### Requirement: Customer Order Checkout
The system SHALL accept customer order placement requests, validate authenticated caller identity forwarded via `X-User-Id` and `X-User-Email`, verify item availability and price integrity against the product service, acquire Midtrans Snap payment tokens upfront, record order records with initial status `PENDING_PAYMENT`, and atomically persist initial `order.created` domain outbox events containing the active `snapRedirectUrl`.

#### Scenario: Successful order placement with immediate payment link
- **WHEN** an authenticated customer sends a valid `POST /api/v1/orders` request containing line items with quantity and shipping address
- **THEN** the system verifies catalog prices, requests a Snap transaction from Midtrans, creates the order and outbox record in a single atomic database transaction, and responds with HTTP 201 containing order ID, order number, Snap token, and Snap redirect URL

#### Scenario: Unauthenticated order placement attempt
- **WHEN** a client sends a `POST /api/v1/orders` request without a valid `X-User-Id` header from the API Gateway
- **THEN** the system rejects the request immediately with HTTP 401 Unauthorized

#### Scenario: Invalid product or price tampering
- **WHEN** a client submits an order with non-existent variant IDs or mismatched quantities
- **THEN** the system rejects the request with HTTP 400 Bad Request and detailed validation errors without creating an order

### Requirement: Customer Order Cancellation
The system SHALL permit customers to cancel their own orders if the order is still in `PENDING_PAYMENT` status, persisting an `order.cancelled` outbox event with complete order metadata.

#### Scenario: Customer cancels unpaid order
- **WHEN** a customer sends `POST /api/v1/orders/{id}/cancel` for an order in `PENDING_PAYMENT` status owned by them
- **THEN** the system transitions the order status to `CANCELLED`, persists an outbox event for `order.cancelled` with order number, customer email, total amount, reason, and line items, and returns HTTP 200

#### Scenario: Customer attempts to cancel already paid order
- **WHEN** a customer sends `POST /api/v1/orders/{id}/cancel` for an order with status `PAID` or `SHIPPED`
- **THEN** the system rejects the cancellation with HTTP 400 Bad Request stating the order cannot be cancelled in its current state

### Requirement: Admin Order Management
The system SHALL provide administrative endpoints guarded by `X-User-Role: admin` to search, list, filter by status, and update the fulfillment state of orders, accepting courier tracking metadata and dispatching lifecycle domain events.

#### Scenario: Admin lists orders with filters
- **WHEN** an authenticated admin sends `GET /api/v1/admin/orders` with optional status, date range, or search parameters
- **THEN** the system returns HTTP 200 with all matching customer orders across the system

#### Scenario: Admin updates fulfillment status to SHIPPED with tracking info
- **WHEN** an admin sends `PATCH /api/v1/admin/orders/{id}/status` with target status `SHIPPED`, `courierName`, and `receiptNumber`
- **THEN** the system updates the order status, persists the courier name and receipt number, creates an outbox event for `order.shipped` containing full shipping details, and returns HTTP 200

#### Scenario: Non-admin attempts admin operation
- **WHEN** a non-admin user requests an admin endpoint
- **THEN** the system returns HTTP 403 Forbidden
