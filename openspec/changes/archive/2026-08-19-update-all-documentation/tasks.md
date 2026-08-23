## 1. Environment & Configuration Documentation

- [x] 1.1 Update `.env.example` to include all runtime environment variables including Redis rate limiting (`REDIS_URL`, `REDIS_PASSWORD`, `REDIS_RATE_LIMIT_ENABLED`), Kafka broker endpoints, and Midtrans credentials with descriptive comments.

## 2. Comprehensive Root README Creation

- [x] 2.1 Author project overview, architecture diagram, and core technology stack in root `README.md`.
- [x] 2.2 Document prerequisites, environment variables, PostgreSQL & Prisma Go setup, and local run steps in `README.md`.
- [x] 2.3 Document Order & Payment lifecycle with Midtrans Snap, Kafka transactional outbox event publisher, and event schemas in `README.md`.
- [x] 2.4 Document Redis rate-limiting rules, developer simulation mode, Docker containerization, and test suite execution in `README.md`.

## 3. OpenAPI Specification Audit & Synchronization

- [x] 3.1 Audit and update `docs/openapi.yaml` to ensure complete route coverage, Gateway auth headers, rate-limiting response headers, and dev simulation endpoints.
- [x] 3.2 Synchronize and format `docs/openapi.json` to guarantee 100% schema parity with `docs/openapi.yaml`.

## 4. Verification and Validation

- [x] 4.1 Verify all registered Chi routes in `internal/router/router.go` match the OpenAPI documentation and README endpoint catalogs.
- [x] 4.2 Validate JSON syntax and structure of `docs/openapi.json`.
