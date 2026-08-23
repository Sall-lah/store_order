## Why

The `store_order` microservice has matured with core architectural components including Chi HTTP routing, Prisma ORM with PostgreSQL, Midtrans Snap payment gateway integration, Kafka transactional outbox background processing, and Redis-backed volumetric and per-user rate limiting. However, the repository currently lacks a root `README.md`, `.env.example` is missing Redis configuration parameters, and the API specifications (`docs/openapi.yaml` and `docs/openapi.json`) need a comprehensive update and synchronization to guarantee full coverage of all routes, headers, rate-limiting response structures, and dev simulation endpoints.

## What Changes

- **Add Root `README.md`**: Create a comprehensive, production-grade guide covering system architecture, technology stack, environment variables, database setup/migrations, Kafka event outbox lifecycle, Midtrans payment integration, Redis rate-limiting rules, interactive documentation (Scalar & Swagger), developer simulation mode, Docker deployment, and testing.
- **Update `.env.example`**: Add complete environment variables including Redis rate-limiter keys (`REDIS_URL`, `REDIS_PASSWORD`, `REDIS_RATE_LIMIT_ENABLED`) with descriptive comments and default values.
- **Sync and Update OpenAPI Documentation**: Audit and update both `docs/openapi.yaml` and `docs/openapi.json` to ensure 100% parity across all endpoints (Customer, Admin, Webhook, Health, Dev Simulation), request/response schemas, security schemes (Gateway offloading headers), and rate limit headers (`X-RateLimit-*`, `Retry-After`).

## Capabilities

### New Capabilities
- `service-documentation`: Comprehensive developer, API, and architectural documentation for the `store_order` microservice, providing unified system documentation, complete environment specifications, and interactive OpenAPI contracts.

### Modified Capabilities
<!-- No modified capabilities; existing functional requirements remain unchanged -->

## Impact

- **Affected Files**:
  - `README.md` (newly created)
  - `.env.example` (updated with Redis configurations)
  - `docs/openapi.yaml` (updated for full parity and schema completeness)
  - `docs/openapi.json` (updated for full parity with YAML specification)
- **APIs and Contracts**: OpenAPI specifications will accurately document all public, admin, webhook, and dev endpoints.
- **Breaking Changes**: None.
