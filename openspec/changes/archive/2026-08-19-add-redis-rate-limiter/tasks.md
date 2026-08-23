## 1. Setup & Configuration

- [x] 1.1 Add `github.com/redis/go-redis/v9` dependency to `go.mod`
- [x] 1.2 Update `internal/config/config.go` with Redis configuration options (`REDIS_URL`, `REDIS_PASSWORD`, `REDIS_RATE_LIMIT_ENABLED`)
- [x] 1.3 Add Redis client lifecycle management and graceful shutdown in `cmd/server/main.go`

## 2. Core Rate Limiting Engine

- [x] 2.1 Implement `internal/ratelimit/limiter.go` with atomic sliding window Lua script and `go-redis` execution
- [x] 2.2 Implement `internal/ratelimit/keys.go` providing `KeyByIP`, `KeyByUser`, and scoped key resolvers
- [x] 2.3 Create `internal/ratelimit/limiter_test.go` with unit tests verifying sliding window math, quota threshold, and fail-open handling

## 3. HTTP Middleware & Router Integration

- [x] 3.1 Implement `internal/middleware/ratelimit.go` to provide Chi-compatible rate limiting middleware with RFC headers (`X-RateLimit-*`, `Retry-After`) and HTTP 429 JSON responses
- [x] 3.2 Register global IP rate limiting middleware in `internal/router/router.go`
- [x] 3.3 Protect checkout (`POST /api/v1/orders`) and cancellation (`POST /api/v1/orders/{id}/cancel`) with fine-grained per-user rate limits
- [x] 3.4 Create `internal/middleware/ratelimit_test.go` verifying HTTP middleware status codes and headers

## 4. Verification & Documentation

- [x] 4.1 Run full project test suite (`go test ./...`) to verify all packages pass cleanly
- [x] 4.2 Update `docs/openapi.yaml` and `docs/openapi.json` with HTTP 429 response definitions on protected endpoints
