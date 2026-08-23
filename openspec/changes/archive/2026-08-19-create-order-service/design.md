## Context

The store microservices platform comprises:
- `store_gateway`: NGINX reverse proxy handling CORS, distributed tracing (`X-Request-ID`), documentation aggregation, and authentication offload (`/_auth_verify` subrequest injecting `X-User-Id`, `X-User-Role`, `X-User-Email`).
- `store_auth`: Go microservice managing RS256 JWKS tokens and user identities.
- `store_product`: Go microservice managing product catalog, pricing, and stock.

`store_order` is the order placement, payment execution, and fulfillment lifecycle engine. It requires tight coordination with `store_product` for catalog verification, integration with Midtrans for Snap payment processing, and event emission to Apache Kafka for asynchronous downstream coordination.

## Goals / Non-Goals

**Goals:**
- Provide customer endpoints for checkout, order history retrieval, order status tracking, and cancellation of unpaid orders.
- Provide admin endpoints to view and transition order fulfillment statuses (`PROCESSING`, `SHIPPED`, `COMPLETED`).
- Integrate with Midtrans Snap API for payment initiation and process incoming webhooks with SHA512 signature validation.
- Implement the Transactional Outbox Pattern with PostgreSQL and a background pure Go worker using `segmentio/kafka-go` to guarantee zero message loss to Kafka topics (`order.created`, `order.paid`, `order.cancelled`, `order.expired`, `order.fulfilled`).
- Support offline local development via `DEV="TRUE"` simulation endpoints and mock Snap tokens.
- Maintain consistency with sibling microservices: Chi router, Prisma Client Go / PostgreSQL, modular architecture, and OpenAPI 3.1 / Scalar / Swagger documentation.

**Non-Goals:**
- Implementing shipping carrier API integrations (e.g. FedEx / JNE tracking webhooks) — manual tracking number input by admin is sufficient.
- Implementing frontend UI components — API only.
- In-memory WebSocket / SSE streaming — standard REST polling and webhook events will be used.

## Decisions

### 1. Synchronous Price Validation + Async Event Choreography (Approach B)
- **Decision**: Validate item prices and product existence via HTTP call against `store_product` synchronously during checkout, while handling post-payment stock deduction and notification workflows asynchronously via Kafka events.
- **Rationale**: Provides instant <200ms checkout response with Midtrans Snap token to customer without requiring client-side WebSocket / polling for inventory reservation, while keeping downstream consumers completely decoupled.
- **Alternative Considered**: Full async saga (publish `order.created` and wait for `inventory.reserved` before issuing Snap token). Rejected due to added UI complexity and polling requirements.

### 2. Pure Go Kafka Client (`segmentio/kafka-go`)
- **Decision**: Use `segmentio/kafka-go` for Kafka writer and reader implementations.
- **Rationale**: Pure Go implementation requires no CGo or `librdkafka` C libraries, resulting in lightweight Docker alpine container builds and fast local compilation without C build toolchain dependencies.
- **Alternative Considered**: `confluent-kafka-go` (requires CGo / librdkafka) and `IBM/sarama`.

### 3. Transactional Outbox Pattern in PostgreSQL
- **Decision**: Write domain events to an `outbox_events` table inside the same PostgreSQL transaction that modifies order status. A dedicated background goroutine polls `outbox_events` and publishes messages to Kafka.
- **Rationale**: Eliminates the dual-write problem (where DB commit succeeds but Kafka network publish fails, or vice versa). Guarantees at-least-once delivery.
- **Alternative Considered**: Direct Kafka publishing in HTTP handler. Rejected because network blips or crashes could cause unrecorded stock deductions or lost payment receipts.

### 4. Dev Simulation Mode (`DEV="TRUE"`)
- **Decision**: Guard dev simulation endpoints (`/api/v1/dev/orders/*`) and mock Snap token generation under `DEV="TRUE"`. In production (`DEV="FALSE"`), these routes are unregistered (HTTP 404).
- **Rationale**: Allows instant end-to-end testing of payment success, cancellation, outbox publishing, and stock deduction without requiring live Midtrans credentials or ngrok webhook tunnels.

## Risks / Trade-offs

- **[Risk] Duplicate Kafka Deliveries**: At-least-once delivery from outbox worker could deliver duplicate `order.paid` events if worker crashes before updating outbox status.
  - **Mitigation**: Events include immutable `event_id` and `order_id`. Downstream consumers (`store_product`) must implement idempotent handling.
- **[Risk] Product Service Latency during Checkout**: Synchronous HTTP call to `store_product` adds network latency.
  - **Mitigation**: Implement tight timeouts (2s) and circuit-breaking / connection reuse with `http.Client`.
- **[Risk] Outbox Table Growth**: Completed events will accumulate in PostgreSQL.
  - **Mitigation**: Implement periodic cleanup job or retention policy deleting `PUBLISHED` events older than 7 days.
