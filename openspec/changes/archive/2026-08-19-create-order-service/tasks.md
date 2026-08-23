## 1. Project Scaffolding & Configuration

- [x] 1.1 Initialize Go module (`github.com/Sall-lah/store_order`) with dependencies (`go-chi/chi/v5`, `segmentio/kafka-go`, `prisma-client-go`, `godotenv`, `shopspring/decimal`)
- [x] 1.2 Create environment configuration loader in `internal/config/config.go` supporting `DEV`, `PORT`, `DATABASE_URL`, `KAFKA_BROKERS`, `MIDTRANS_SERVER_KEY`, `MIDTRANS_CLIENT_KEY`, `MIDTRANS_IS_PRODUCTION`, `PRODUCT_SERVICE_URL`
- [x] 1.3 Create baseline HTTP middlewares (`internal/middleware`) for Request ID tracing (`X-Request-ID`), Gateway identity extraction (`X-User-Id`, `X-User-Role`, `X-User-Email`), RequireAdmin guard, and CORS/JSON recovery
- [x] 1.4 Create `Dockerfile` and `.dockerignore` for multi-stage Go compilation

## 2. Database Schema & Prisma ORM

- [x] 2.1 Define Prisma schema in `prisma/schema.prisma` with models: `Order`, `OrderItem`, `OutboxEvent`, and enums (`OrderStatus`, `OutboxStatus`)
- [x] 2.2 Generate Prisma Go client into `internal/db` and setup database connection lifecycle in `internal/db/client.go`
- [x] 2.3 Implement repository layer in `internal/repository` for atomic order creation, state transitions, order querying, and outbox event batching

## 3. External Integrations (Product Service & Midtrans)

- [x] 3.1 Implement Product Service HTTP client in `internal/integration/product` to validate product/variant existence and fetch authoritative prices
- [x] 3.2 Implement Midtrans Snap client in `internal/integration/midtrans` to generate Snap transaction tokens and redirect URLs
- [x] 3.3 Implement Midtrans webhook payload parser and SHA512 signature verifier in `internal/integration/midtrans/signature.go`

## 4. Kafka Producer & Transactional Outbox Worker

- [x] 4.1 Implement Kafka writer wrapper in `internal/kafka/producer.go` using pure Go `segmentio/kafka-go`
- [x] 4.2 Implement Outbox background worker in `internal/outbox/worker.go` to poll pending events, publish to Kafka topics (`order.created`, `order.paid`, `order.cancelled`, `order.expired`, `order.fulfilled`), and mark published
- [x] 4.3 Add graceful shutdown orchestration to cleanly flush outbox batches and close Kafka connections on SIGINT/SIGTERM

## 5. Order Business Logic & API Handlers

- [x] 5.1 Implement Order Service in `internal/service/order_service.go` orchestrating price validation, order persistence, Snap token generation, and atomic outbox insertions
- [x] 5.2 Implement Customer HTTP handlers in `internal/handler/order_handler.go` for `POST /api/v1/orders`, `GET /api/v1/orders`, `GET /api/v1/orders/{id}`, and `POST /api/v1/orders/{id}/cancel`
- [x] 5.3 Implement Midtrans Webhook handler in `internal/handler/webhook_handler.go` for `POST /api/v1/orders/webhook/midtrans`
- [x] 5.4 Implement Admin HTTP handlers in `internal/handler/admin_handler.go` for `GET /api/v1/admin/orders` and `PATCH /api/v1/admin/orders/{id}/status`

## 6. Dev Simulation Mode

- [x] 6.1 Implement Dev Simulation service and handlers in `internal/handler/dev_handler.go` for `/api/v1/dev/orders/{id}/simulate-success`, `simulate-cancel`, and `simulate-expire`
- [x] 6.2 Implement Mock Snap Token provider when `DEV="TRUE"` and Midtrans credentials are unset
- [x] 6.3 Register dev routes conditionally in `internal/router/router.go` only when `DEV="TRUE"`, returning 404 in production

## 7. OpenAPI Documentation & Health Checks

- [x] 7.1 Author OpenAPI 3.1 specification in `docs/openapi.yaml` and generate JSON format in `docs/openapi.json`
- [x] 7.2 Mount Scalar UI (`/docs/`), Swagger UI (`/swagger/`), and raw OpenAPI specs endpoints
- [x] 7.3 Implement `/health` probe returning service liveness, database status, and Kafka connectivity

## 8. Verification & Automated Tests

- [x] 8.1 Write unit tests for Midtrans SHA512 signature verification
- [x] 8.2 Write unit tests for Order Service state transitions and outbox event persistence
- [x] 8.3 Write HTTP handler integration tests for checkout, authorization guards, and dev simulation endpoints
