## ADDED Requirements

### Requirement: Customer Order Checkout
The system SHALL accept customer order placement requests, validate authenticated caller identity forwarded via `X-User-Id` and `X-User-Email`, verify item availability and price integrity against the product service, record order records with initial status `PENDING_PAYMENT`, and return payment initiation tokens.

#### Scenario: Successful order placement
- **WHEN** an authenticated customer sends a valid `POST /api/v1/orders` request containing line items with quantity and shipping address
- **THEN** the system validates pricing with the product catalog, generates an atomic order record in status `PENDING_PAYMENT`, creates line items, and responds with HTTP 201 containing order ID, order number, and Midtrans Snap token

#### Scenario: Unauthenticated order placement attempt
- **WHEN** a client sends a `POST /api/v1/orders` request without a valid `X-User-Id` header from the API Gateway
- **THEN** the system rejects the request immediately with HTTP 401 Unauthorized

#### Scenario: Invalid product or price tampering
- **WHEN** a client submits an order with non-existent variant IDs or mismatched quantities
- **THEN** the system rejects the request with HTTP 400 Bad Request and detailed validation errors without creating an order

### Requirement: Customer Order Retrieval
The system SHALL allow authenticated customers to query their personal order history with pagination and view details for individual orders they own.

#### Scenario: Customer lists own orders
- **WHEN** an authenticated customer sends a `GET /api/v1/orders` request
- **THEN** the system returns a paginated list of orders strictly matching the customer's `X-User-Id` ordered by creation date descending

#### Scenario: Customer views specific order
- **WHEN** an authenticated customer sends a `GET /api/v1/orders/{id}` request for an order matching their `X-User-Id`
- **THEN** the system returns HTTP 200 with the full order breakdown including line items, prices, status, and payment metadata

#### Scenario: Customer attempts to view another user's order
- **WHEN** a customer attempts to view an order belonging to a different `user_id`
- **THEN** the system returns HTTP 404 Not Found or HTTP 403 Forbidden to prevent unauthorized access

### Requirement: Customer Order Cancellation
The system SHALL permit customers to cancel their own orders if the order is still in `PENDING_PAYMENT` status.

#### Scenario: Customer cancels unpaid order
- **WHEN** a customer sends `POST /api/v1/orders/{id}/cancel` for an order in `PENDING_PAYMENT` status owned by them
- **THEN** the system transitions the order status to `CANCELLED`, persists an outbox event for `order.cancelled`, and returns HTTP 200

#### Scenario: Customer attempts to cancel already paid order
- **WHEN** a customer sends `POST /api/v1/orders/{id}/cancel` for an order with status `PAID` or `SHIPPED`
- **THEN** the system rejects the cancellation with HTTP 400 Bad Request stating the order cannot be cancelled in its current state

### Requirement: Admin Order Management
The system SHALL provide administrative endpoints guarded by `X-User-Role: admin` to search, list, filter by status, and update the fulfillment state of orders.

#### Scenario: Admin lists orders with filters
- **WHEN** an authenticated admin sends `GET /api/v1/admin/orders` with optional status, date range, or search parameters
- **THEN** the system returns HTTP 200 with all matching customer orders across the system

#### Scenario: Admin updates fulfillment status
- **WHEN** an admin sends `PATCH /api/v1/admin/orders/{id}/status` with target status `PROCESSING`, `SHIPPED`, or `COMPLETED`
- **THEN** the system validates valid state transition rules, updates the order status, records an outbox event, and returns HTTP 200

#### Scenario: Non-admin attempts admin operation
- **WHEN** a non-admin user requests an admin endpoint
- **THEN** the system returns HTTP 403 Forbidden
