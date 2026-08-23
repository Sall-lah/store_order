## Why

The upstream API Gateway currently does not enforce rate limiting. As a result, the `store_order` service is directly vulnerable to bot abuse, inventory scalping on checkout, infinite polling loops, and rapid concurrent duplicate requests that could overload downstream services (Product Service, Midtrans, PostgreSQL, and Kafka). Introducing Redis-backed rate limiting ensures self-defending service boundaries with fine-grained per-IP and per-user throttling.

## What Changes

- **Add Redis Client Integration**: Configure `go-redis/v9` client connection pool with health checks and graceful fail-open resilience.
- **Add Sliding Window Rate Limiting Engine**: Implement an atomic Lua script in Redis using Sorted Sets (`ZSET`) to compute sliding window quotas and retry delays.
- **Add HTTP Rate Limiting Middleware**: Implement Chi HTTP middleware supporting configurable keys (by IP, by User ID, or by Route prefix), returning standard HTTP `429 Too Many Requests` responses with `Retry-After` and `X-RateLimit-*` headers.
- **Apply Rate Limiting Tiers**:
  - Global IP limiter: 120 req/min across all endpoints.
  - Checkout limiter: 3 requests per 10 seconds per authenticated user on `POST /api/v1/orders`.
  - Cancellation limiter: 5 requests per minute per authenticated user on `POST /api/v1/orders/{id}/cancel`.
  - Public Webhook limiter: 300 requests per minute per source IP on `POST /api/v1/orders/webhook/midtrans`.

## Capabilities

### New Capabilities
- `redis-rate-limiter`: Redis-backed sliding window rate limiting middleware and quota management for `store_order` endpoints.

### Modified Capabilities
<!-- None: existing business requirements for order-management, dev-simulation, kafka-outbox, and midtrans-payment remain unchanged -->

## Impact

- **Dependencies**: Adds `github.com/redis/go-redis/v9` to `go.mod`.
- **Configuration**: Adds `REDIS_URL`, `REDIS_PASSWORD`, and `REDIS_RATE_LIMIT_ENABLED` to `internal/config`.
- **Router / Middleware**: Updates `internal/router/router.go` and `internal/middleware` to inject rate limiter middleware.
- **Runtime**: Adds Redis dependency to application bootstrap with fail-open fallback if Redis is unreachable.
