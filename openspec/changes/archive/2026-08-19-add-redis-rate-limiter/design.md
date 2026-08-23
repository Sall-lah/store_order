## Context

The `store_order` microservice operates downstream from an API Gateway that currently lacks edge rate limiting. Consequently, all traffic hits `store_order` directly. Sensitive endpoints such as checkout (`POST /api/v1/orders`) trigger heavy workflows (database transactions, Product Service inventory locks, Midtrans Snap token generation, and Kafka outbox enqueueing). Without rate limiting, malicious actors or rapid client retries can cause race conditions, inventory exhaustion, or third-party quota saturation.

## Goals / Non-Goals

**Goals:**
- Provide a modular, high-performance Redis sliding-window rate limiter in Go (`internal/ratelimit`).
- Implement Chi-compatible HTTP middleware with configurable keying (by IP, by User ID, or prefixed).
- Return standard RFC-compliant HTTP `429 Too Many Requests` responses with `Retry-After`, `X-RateLimit-Limit`, and `X-RateLimit-Remaining` headers.
- Guarantee high service availability by failing open when Redis is slow (>25ms) or unavailable.
- Provide unit tests and mock integration tests.

**Non-Goals:**
- Replacing edge DDoS / WAF mitigation (e.g. Cloudflare / AWS Shield) for raw network floods (L3/L4).
- Distributed rate synchronization across multi-region datacenters without a shared Redis instance.

## Decisions

### 1. Algorithm: Sliding Window Log via Redis Sorted Sets (ZSET)
- **Decision**: Use an atomic Redis Lua script executing `ZREMRANGEBYSCORE`, `ZCARD`, `ZADD`, and `PEXPIRE`.
- **Why**: Eliminates boundary burst vulnerabilities present in Fixed Window counters.
- **Alternatives Considered**:
  - *Fixed Window*: Vulnerable to 2x traffic bursts at boundary resets.
  - *Token Bucket*: Requires more complex state synchronization for dynamic window queries.

### 2. Client Library: `go-redis/v9`
- **Decision**: Integrate `github.com/redis/go-redis/v9` into `internal/config` and initialization lifecycle.
- **Why**: Official, active, robust connection pooling, cluster/sentinel support, and SHA1-cached Lua script execution (`EvalSha`).

### 3. Fail-Open Architecture with Context Timeout
- **Decision**: Enforce a strict 25ms timeout on Redis rate limit evaluation. If the timeout triggers or Redis returns an error, the middleware logs a warning, appends `X-RateLimit-Degraded: true` header, and calls `next.ServeHTTP(w, r)`.
- **Why**: E-commerce revenue and availability must not be compromised during transient cache degradation.

### 4. Pluggable Keyer Strategy
- **Decision**: Provide functional key resolvers:
  - `KeyByIP`: Uses `chiMiddleware.RealIP` or `r.RemoteAddr`.
  - `KeyByUser`: Uses [`middleware.GetUserContext(r.Context()).UserID`](file:///C:/Users/LENOVO/Documents/VsCode/GitHub/store_order/internal/middleware/auth.go#L25), falling back to IP if unauthenticated.
  - `KeyByUserWithPrefix(prefix)`: Scopes limits per route (e.g. `ratelimit:checkout:user:<id>`).

## Risks / Trade-offs

- **[Redis Memory Growth with ZSET]** → Mitigation: Keys are automatically expired using `PEXPIRE` with the sliding window duration. Even with thousands of requests, memory footprint remains minimal (<1KB per active user).
- **[Redis Unavailability / Network Partition]** → Mitigation: Fail-open design with strict 25ms context timeout prevents request backlog or HTTP 500 errors during Redis downtime.
- **[NAT / Corporate Shared IP Collisions]** → Mitigation: Global IP limits are set generously (120 req/min), while strict limits (3 req/10s) are applied exclusively per authenticated `user_id`.

## Migration Plan

1. Add `REDIS_URL`, `REDIS_PASSWORD`, and `REDIS_RATE_LIMIT_ENABLED` to environment configuration.
2. Initialize Redis client at server startup in `cmd/server/main.go`.
3. Register global IP limiter and endpoint-specific limiters in `internal/router/router.go`.
4. Deploy service; if needed, disable rate limiting instantly via `REDIS_RATE_LIMIT_ENABLED=false` without code rollback.

## Open Questions

- None blocking: Standard Redis instance (`localhost:6379`) is already configured locally by user.
