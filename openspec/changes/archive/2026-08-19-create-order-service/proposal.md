## Why

The store ecosystem currently includes authentication (`store_auth`), product catalog (`store_product`), and an API gateway (`store_gateway`), but lacks an order processing and payment execution engine. Introducing `store_order` provides a resilient, event-driven order lifecycle service capable of processing customer checkouts, validating item prices against the product service, facilitating secure payments via Midtrans, and asynchronously coordinating inventory and notifications through Apache Kafka.

## What Changes

- **Order Ingress & Lifecycle Management**: Provide customer endpoints to place orders, list personal order history, and view order details, alongside admin endpoints to monitor and update fulfillment states.
- **Midtrans Payment Integration**: Generate Midtrans Snap transaction tokens and redirect URLs, and verify incoming Midtrans webhook notifications with SHA512 signature authentication.
- **Transactional Outbox & Kafka Pipeline**: Guarantee event publication (`order.paid`, `order.cancelled`, `order.expired`, `order.fulfilled`) to Kafka using PostgreSQL transactional outbox tables and a pure Go background publisher worker (`segmentio/kafka-go`).
- **Developer Simulation Mode**: Provide test simulation endpoints when `DEV="TRUE"` is set in `.env` to simulate payment success, cancellation, and expiration offline without live Midtrans credentials.
- **Gateway & Documentation Integration**: Conform to `store_gateway` authentication offloading (`X-User-Id`, `X-User-Role`, `X-User-Email`) and expose OpenAPI 3.1, Scalar UI, and Swagger UI documentation proxies.

## Capabilities

### New Capabilities
- `order-management`: Core order creation, price verification against product catalog, ownership checks, status transitions, and administrative order management.
- `midtrans-payment`: Midtrans Snap token generation, payment redirect handling, and secure webhook notification processing with SHA512 signature validation.
- `kafka-outbox`: Transactional Outbox event persistence in PostgreSQL, background outbox worker for reliable at-least-once message publication to Kafka topics, and graceful shutdown.
- `dev-simulation`: Offline developer simulation endpoints and mock Snap provider active only when `DEV="TRUE"`.

### Modified Capabilities
<!-- None. This is a greenfield microservice. -->

## Impact

- **APIs**: Exposes `/api/v1/orders/*`, `/api/v1/admin/orders/*`, `/api/v1/dev/orders/*` (dev-only), and documentation routes (`/docs/*`, `/swagger/*`).
- **Dependencies**: Go 1.26+, Chi router (`github.com/go-chi/chi/v5`), Prisma Client Go (`github.com/steebchen/prisma-client-go`), pure Go Kafka (`github.com/segmentio/kafka-go`), PostgreSQL, Redis.
- **Ecosystem**: Integrates with `store_gateway` for auth offloading, interacts with `store_product` for catalog price validation and stock event consumption, and provides event bus streams for downstream notification and analytics services.
