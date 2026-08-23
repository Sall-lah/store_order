## Context

The `store_order` service is a Go microservice responsible for order creation, Midtrans Snap payment integration, order lifecycle status management (Pending, Paid, Cancelled, Expired, Fulfilled), transactional outbox event publishing via Apache Kafka, and rate limiting with Redis.

While the codebase is modular and fully implemented across `internal/` packages, the repository is missing a primary `README.md`, `.env.example` lacks Redis configuration variables, and the OpenAPI documentation (`docs/openapi.yaml` and `docs/openapi.json`) requires a full audit and synchronization against the routes defined in `internal/router/router.go`.

## Goals / Non-Goals

**Goals:**
- Author a comprehensive root `README.md` detailing system architecture, tech stack, data models, transactional outbox worker lifecycle, payment integration with Midtrans Snap, Redis rate-limiting rules, developer simulation endpoints, local setup, database migrations via Prisma, testing, and Docker execution.
- Update `.env.example` to include all active configuration parameters, specifically `REDIS_URL`, `REDIS_PASSWORD`, and `REDIS_RATE_LIMIT_ENABLED`.
- Ensure complete parity between `docs/openapi.yaml` and `docs/openapi.json` covering all routes (Health, Customer Orders, Midtrans Webhook, Admin Operations, and Developer Simulation), Gateway authentication headers (`X-User-Id`, `X-User-Role`, `X-User-Email`), rate limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`), and error response schemas.

**Non-Goals:**
- Modifying core application business logic or altering HTTP route behaviors.
- Modifying the Prisma database schema or running destructive database migrations.

## Decisions

### 1. Dual OpenAPI Representation (YAML & JSON)
- **Rationale**: The Go router serves both `/docs/openapi.yaml` and `/docs/openapi.json`, and the embedded Scalar UI and Swagger UI rely on `/docs/openapi.json`. Maintaining exact schema parity between both files ensures external API tooling and embedded documentation viewers function seamlessly without schema drift.
- **Alternatives Considered**: Serving only YAML or on-the-fly converting YAML to JSON in memory. Pre-synchronized static files ensure zero runtime conversion overhead and minimal dependencies.

### 2. Comprehensive Architectural & Operational README
- **Rationale**: Developers and operators need a single source of truth for understanding how the service interacts with the API gateway (`store_gateway`), payment provider (Midtrans), Kafka broker, and PostgreSQL database. Including Mermaid flowcharts for the checkout lifecycle and transactional outbox ensures clarity.
- **Alternatives Considered**: Minimal quickstart README. Rejected because microservice event flows and gateway authentication headers require explicit architectural context.

### 3. Clear Categorization in OpenAPI Schemas
- **Rationale**: Group endpoints into distinct tags: `Health`, `Customer Orders`, `Payment Webhooks`, `Admin Operations`, and `Developer Simulation (Dev Only)`. Clearly document Gateway Offloading security headers.

## Risks / Trade-offs

- **[Risk] OpenAPI specification drift from router implementations** → **Mitigation**: Perform a line-by-line route audit against `internal/router/router.go` and verify all query parameters, URL path parameters, and payload schemas.
- **[Risk] Confusion regarding Gateway Offloading authentication** → **Mitigation**: Explicitly document that client authentication is offloaded to `store_gateway`, which forwards validated identity headers (`X-User-Id`, `X-User-Role`).
