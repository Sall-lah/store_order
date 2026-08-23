## ADDED Requirements

### Requirement: Global IP Rate Limiting
The system SHALL limit excessive HTTP requests originating from a single client IP address across all incoming routes.

#### Scenario: Request within global IP quota
- **WHEN** a client IP sends requests within the configured threshold (e.g. up to 120 requests in 1 minute)
- **THEN** the system SHALL allow the request to proceed and return standard HTTP response with updated `X-RateLimit-*` headers

#### Scenario: Request exceeding global IP quota
- **WHEN** a client IP sends more than the allowed requests within the 1-minute window
- **THEN** the system SHALL reject the request with HTTP `429 Too Many Requests` and include a `Retry-After` header indicating the wait duration in seconds

### Requirement: Granular Checkout Rate Limiting
The system SHALL enforce strict per-user rate limits on the checkout creation endpoint (`POST /api/v1/orders`) to prevent race conditions, duplicate charges, and inventory locking abuse.

#### Scenario: User submits checkout within quota
- **WHEN** an authenticated user submits up to 3 checkout requests within a 10-second sliding window
- **THEN** the system SHALL allow the checkout request to be processed by the order service

#### Scenario: User exceeds checkout quota
- **WHEN** an authenticated user submits a 4th checkout request within the same 10-second sliding window
- **THEN** the system SHALL immediately reject the request with HTTP `429 Too Many Requests` without contacting the Product Service or Midtrans API

### Requirement: Granular Order Cancellation Rate Limiting
The system SHALL throttle order cancellation requests (`POST /api/v1/orders/{id}/cancel`) per authenticated user.

#### Scenario: User cancels within rate limit
- **WHEN** an authenticated user issues cancellation requests within the quota (e.g. 5 requests per minute)
- **THEN** the system SHALL process the cancellation normally

#### Scenario: User exceeds cancellation limit
- **WHEN** an authenticated user submits more than 5 cancellation requests within a 1-minute window
- **THEN** the system SHALL respond with HTTP `429 Too Many Requests` and a descriptive JSON error payload

### Requirement: Fail-Open Resilience on Redis Outage
The rate limiting middleware SHALL fail open if Redis is unreachable, timed out (>25ms), or returns an operational error.

#### Scenario: Redis connection drops or times out
- **WHEN** a request arrives while Redis is disconnected or taking longer than 25ms to respond
- **THEN** the system SHALL log a warning, set `X-RateLimit-Degraded: true` header, and allow the request to reach the underlying route handler without blocking user traffic

### Requirement: Configurable Keying Strategies
The system SHALL support multiple key generation strategies based on request context, including IP-based keying, User ID-based keying, and endpoint-specific scoped prefixes.

#### Scenario: Authenticated user key resolution
- **WHEN** a request is evaluated with `KeyByUser` strategy and contains valid user identity context
- **THEN** the system SHALL construct the Redis key using the user ID (e.g. `ratelimit:checkout:user:<id>`)

#### Scenario: Unauthenticated request fallback
- **WHEN** a request is evaluated with `KeyByUser` strategy but lacks user identity context
- **THEN** the system SHALL fall back to client IP keying (e.g. `ratelimit:checkout:ip:<ip>`)
