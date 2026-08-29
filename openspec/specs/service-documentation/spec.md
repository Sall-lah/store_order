# Service Documentation Specification

## Purpose
Provides comprehensive developer, API, and architectural documentation for the `store_order` microservice, guaranteeing synchronized OpenAPI contracts, complete environment specifications, and clear development guides.
## Requirements
### Requirement: Comprehensive Project README Documentation
The system SHALL provide a comprehensive, production-grade `README.md` at the project root documenting architecture, technology stack, transactional outbox pattern, rate-limiting policies, Midtrans payment workflows, database migrations with Prisma, testing, and container deployment.

#### Scenario: Developer views repository README
- **WHEN** a developer or contributor opens the project root `README.md`
- **THEN** the documentation SHALL clearly present the service architecture, prerequisites, configuration options, directory structure, API endpoint overview, and local development instructions

#### Scenario: Developer follows local setup and testing guides
- **WHEN** a developer executes setup commands from the `README.md`
- **THEN** the instructions SHALL provide exact steps to install dependencies, run Prisma migrations, start local Kafka/PostgreSQL/Redis services, execute unit tests, and launch the HTTP server

### Requirement: Complete Environment Configuration Specification
The system SHALL maintain a synchronized and fully commented `.env.example` file that enumerates all configurable environment variables, including Redis rate limiter, Kafka brokers, Midtrans credentials, database URL, and developer simulation flags.

#### Scenario: Developer inspects environment variables template
- **WHEN** a developer reviews `.env.example`
- **THEN** the file SHALL define `PORT`, `DEV`, `ENABLE_DOCS`, `DATABASE_URL`, `KAFKA_BROKERS`, `PRODUCT_SERVICE_URL`, `MIDTRANS_SERVER_KEY`, `MIDTRANS_CLIENT_KEY`, `MIDTRANS_IS_PRODUCTION`, `REDIS_URL`, `REDIS_PASSWORD`, and `REDIS_RATE_LIMIT_ENABLED` with appropriate default development values

### Requirement: OpenAPI Specification Parity and Interactive Documentation
The system SHALL maintain synchronized and valid OpenAPI 3.1 specifications in both YAML (`docs/openapi.yaml`) and JSON (`docs/openapi.json`) formats covering 100% of available HTTP routes, authentication headers, rate limit response headers, error models, dev simulation endpoints, and relative server definitions (`./`).

#### Scenario: Client queries OpenAPI spec endpoints
- **WHEN** a client or automated tool fetches `/docs/openapi.yaml` or `/docs/openapi.json`
- **THEN** the returned specification SHALL accurately describe all paths (`/health`, `/api/v1/orders`, `/api/v1/orders/{id}`, `/api/v1/orders/{id}/cancel`, `/api/v1/orders/webhook/midtrans`, `/api/v1/admin/orders`, `/api/v1/admin/orders/{id}/status`, `/api/v1/dev/orders/*`) with complete parameter and response schemas, and SHALL include a relative server URL (`./`) in the `servers` configuration

#### Scenario: Developer navigates interactive API documentation
- **WHEN** a developer accesses `/docs` (Scalar UI) or `/swagger` (Swagger UI) in a browser while `ENABLE_DOCS=true`
- **THEN** the interactive UI SHALL render all documented endpoints with schema validation, example payloads, and authentication parameter descriptions, AND SHALL load the OpenAPI specification via relative path references (`./openapi.json` or relative paths) to support arbitrary reverse proxy prefixes and sub-paths

#### Scenario: Trailing slash normalization for documentation
- **WHEN** a client accesses `/docs` without a trailing slash
- **THEN** the router SHALL redirect or normalize the request to `/docs/` (or ensure RFC 3986 path resolution targets `/docs/openapi.json`)

