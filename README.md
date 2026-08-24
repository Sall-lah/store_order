# Store Order Microservice (`store_order`)

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Router](https://img.shields.io/badge/Router-Chi%20v5-blue)](https://github.com/go-chi/chi)
[![Database](https://img.shields.io/badge/Database-PostgreSQL-336791?logo=postgresql)](https://www.postgresql.org/)
[![ORM](https://img.shields.io/badge/ORM-Prisma%20Go%20Client-2D3748?logo=prisma)](https://github.com/steebchen/prisma-client-go)
[![Event Streaming](https://img.shields.io/badge/Streaming-Apache%20Kafka-231F20?logo=apachekafka)](https://kafka.apache.org/)
[![Cache & Rate Limit](https://img.shields.io/badge/Rate%20Limit-Redis-DC382D?logo=redis)](https://redis.io/)
[![Payment](https://img.shields.io/badge/Payment-Midtrans%20Snap-002F6C)](https://midtrans.com/)

A production-grade, event-driven Order Management and Payment Processing microservice built in Go. It manages the complete lifecycle of customer orders, orchestrates Midtrans Snap payment tokens and webhook notifications, publishes domain events via the Transactional Outbox Pattern to Apache Kafka, and defends against volumetric abuse using Redis sliding-window rate limiting.

---

## 📑 Table of Contents

- [Architecture Overview](#-architecture-overview)
- [Key Features](#-key-features)
- [Technology Stack](#-technology-stack)
- [Repository Structure](#-repository-structure)
- [Prerequisites & Environment Configuration](#-prerequisites--environment-configuration)
- [Database Setup & Prisma ORM](#-database-setup--prisma-orm)
- [Getting Started (Local Development)](#-getting-started-local-development)
- [API Endpoints & Documentation](#-api-endpoints--documentation)
- [Transactional Outbox & Kafka Pipeline](#-transactional-outbox--kafka-pipeline)
- [Redis Rate Limiting Rules](#-redis-rate-limiting-rules)
- [Developer Simulation Mode](#-developer-simulation-mode)
- [Testing](#-testing)
- [Docker Deployment](#-docker-deployment)

---

## 🏗 Architecture Overview

```mermaid
flowchart TD
    Client[Client / Frontend] -->|HTTP Requests| Gateway[API Gateway / store_gateway]
    Gateway -->|Forward with X-User-Id / X-User-Role| Router[Chi HTTP Router]
    
    subgraph store_order ["Store Order Service"]
        Router --> Middleware[RateLimit & Auth Middleware]
        Middleware --> Handlers[HTTP Handlers: Order, Webhook, Admin, Dev]
        Handlers --> Service[Order Service]
        Service --> OrderRepo[Order Repository]
        Service --> OutboxRepo[Outbox Repository]
        Service --> ProductClient[Product Service Client]
        Service --> MidtransClient[Midtrans Snap Client]
        
        OutboxWorker[Transactional Outbox Worker] --> OutboxRepo
        UserConsumer[User Event Kafka Consumer] --> Service
    end

    OrderRepo -->|Prisma Client| Postgres[(PostgreSQL DB)]
    OutboxRepo -->|Prisma Client| Postgres
    Middleware -->|Sliding Window Counter| Redis[(Redis)]
    ProductClient -->|HTTP GET /products| ProductSvc[Product Microservice]
    MidtransClient -->|HTTP POST /snap/v1/transactions| MidtransAPI[Midtrans Gateway]
    OutboxWorker -->|Publish Events: order.events| KafkaOut[Apache Kafka Outbound]
    KafkaIn[Apache Kafka: user.events] -->|user.deleted / user.banned| UserConsumer
```

### Order Lifecycle State Machine

```mermaid
stateDiagram-v2
    [*] --> PENDING_PAYMENT: Checkout Created
    PENDING_PAYMENT --> PAID: Webhook Settlement / Dev Simulation
    PENDING_PAYMENT --> CANCELLED: Customer / Admin Cancel / Dev Simulation / User Ban or Delete
    PENDING_PAYMENT --> EXPIRED: Payment Window Timeout / Midtrans Expiry
    PAID --> PROCESSING: Admin Status Update
    PROCESSING --> SHIPPED: Admin Status Update
    SHIPPED --> COMPLETED: Delivery Confirmation
    CANCELLED --> [*]
    EXPIRED --> [*]
    COMPLETED --> [*]
```

---

## 🌟 Key Features

1. **Atomic Checkout & Price Verification**: Validates product prices and stock availability against `store_product` before atomically persisting order records.
2. **Midtrans Snap Integration**: Generates Snap payment tokens and redirect URLs, and verifies webhook notification signatures (SHA-512) to securely settle, expire, or cancel orders.
3. **Transactional Outbox Pattern**: Order state mutations and domain events are written to the database within the same transaction, guaranteeing zero message loss when streaming to Apache Kafka.
4. **Resilient Rate Limiting**: Multi-tiered rate limiting with Redis sliding-window counters (IP global, per-user checkout, per-user cancellation, webhook source rate limiting) with a **fail-open** 25ms circuit breaker.
5. **API Gateway Offloading Authentication**: Seamlessly consumes verified upstream gateway headers (`X-User-Id`, `X-User-Email`, `X-User-Role`).
6. **Embedded Interactive Documentation**: Live OpenAPI 3.1 documentation rendered via **Scalar UI** (`/docs`) and **Swagger UI** (`/swagger`).
7. **Developer Simulation Mode**: Fast-track testing endpoints (`DEV=true`) allowing instant simulation of payment settlement, cancellation, and expiration without external webhook dependencies.
8. **User Lifecycle Event Handling (`user.events`)**:
   - **`user.deleted` (GDPR Right to be Forgotten)**: Auto-cancels in-flight `PENDING_PAYMENT` orders, redacts PII (`userEmail` masked, `shippingAddress` pseudonymized to `[ANONYMIZED]`, tokens cleared), and writes `order.cancelled` outbox events to release reserved stock in `store_product`.
   - **`user.banned` (Fraud & Abuse Defense)**: Auto-cancels in-flight `PENDING_PAYMENT` orders to release inventory, while strictly preserving all customer PII, email, and shipping address records for forensic analysis and dispute defense.

---

## 🛠 Technology Stack

- **Language**: Go 1.26+
- **HTTP Routing**: [Chi v5](https://github.com/go-chi/chi) with CORS & logging middlewares
- **ORM & Data Layer**: [Prisma Client Go](https://github.com/steebchen/prisma-client-go) with PostgreSQL
- **Event Streaming**: [segmentio/kafka-go](https://github.com/segmentio/kafka-go)
- **Caching & Rate Limiting**: [go-redis/v9](https://github.com/redis/go-redis)
- **Payment Gateway**: Midtrans Snap API
- **API Documentation**: OpenAPI 3.1, [Scalar](https://github.com/scalar/scalar), [Swagger UI](https://swagger.io/tools/swagger-ui/)
- **Containerization**: Multi-stage Alpine Dockerfile

---

## 📁 Repository Structure

```
store_order/
├── cmd/
│   └── server/
│       └── main.go                 # Application bootstrap & dependency injection
├── docs/
│   ├── openapi.json                # OpenAPI 3.1 specification (JSON format)
│   └── openapi.yaml                # OpenAPI 3.1 specification (YAML format)
├── internal/
│   ├── config/                     # Environment variable parsing and validation
│   ├── db/                         # Generated Prisma Client Go engine & models
│   ├── handler/                    # HTTP controllers (Order, Admin, Webhook, Dev, Health)
│   ├── integration/
│   │   ├── midtrans/               # Midtrans Snap API client & SHA-512 signature verification
│   │   └── product/                # External product microservice HTTP client
│   ├── kafka/                      # Kafka producer abstraction
│   ├── middleware/                 # Auth extraction, RBAC guard, Rate limiter, Logger
│   ├── outbox/                     # Background Transactional Outbox worker
│   ├── ratelimit/                  # Sliding-window Redis rate limiter implementation
│   ├── repository/                 # PostgreSQL database data access layer
│   ├── router/                     # Chi multiplexer routing and doc endpoint registrations
│   └── service/                    # Core business logic and payment lifecycle orchestration
├── openspec/                       # OpenSpec specifications and planning artifacts
├── prisma/
│   └── schema.prisma               # Prisma schema definition
├── Dockerfile                      # Multi-stage container build definition
├── go.mod / go.sum                 # Go module definitions
└── .env.example                    # Environment variable configuration template
```

---

## ⚙️ Prerequisites & Environment Configuration

### Prerequisites
- **Go**: Version 1.26 or higher
- **PostgreSQL**: Version 14 or higher
- **Apache Kafka**: Version 3.x+
- **Redis**: Version 7.x+
- **Prisma CLI**: For schema migrations (`npm install -g prisma`)

### Configuration Options (`.env`)

Copy the example configuration file:
```bash
cp .env.example .env
```

| Variable | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `PORT` | `string` | `8060` | HTTP port for the microservice |
| `DEV` | `bool` | `false` | Enables developer simulation endpoints & mock mode |
| `ENABLE_DOCS` | `bool` | `true` | Enables `/docs` (Scalar) and `/swagger` (Swagger UI) |
| `DATABASE_URL` | `string` | *(Required)* | PostgreSQL connection string with schema parameter |
| `KAFKA_BROKERS` | `string` | `localhost:9092` | Comma-separated list of Kafka broker addresses |
| `PRODUCT_SERVICE_URL` | `string` | `http://localhost:8040` | Base URL of the Product Microservice |
| `MIDTRANS_SERVER_KEY` | `string` | *(Required)* | Midtrans server secret key |
| `MIDTRANS_CLIENT_KEY` | `string` | *(Optional)* | Midtrans client key |
| `MIDTRANS_IS_PRODUCTION`| `bool` | `false` | Set `true` for Midtrans Production environment |
| `REDIS_URL` | `string` | `localhost:6379` | Redis host and port for distributed rate limiting |
| `REDIS_PASSWORD` | `string` | `""` | Optional password for Redis authentication |
| `REDIS_RATE_LIMIT_ENABLED`| `bool` | `true` | Set `false` to bypass Redis rate limiting |

---

## 🗄 Database Setup & Prisma ORM

The project uses Prisma schema (`prisma/schema.prisma`) to maintain models and generate the Go client into `internal/db`.

1. **Push Schema to PostgreSQL Database**:
   ```bash
   npx prisma db push --schema=./prisma/schema.prisma
   ```

2. **Generate Go Client**:
   ```bash
   go run github.com/steebchen/prisma-client-go generate --schema=./prisma/schema.prisma
   ```

---

## 🚀 Getting Started (Local Development)

1. **Clone the repository**:
   ```bash
   git clone https://github.com/Sall-lah/store_order.git
   cd store_order
   ```

2. **Install Go Dependencies**:
   ```bash
   go mod download
   ```

3. **Configure Environment Variables**:
   ```bash
   cp .env.example .env
   # Edit .env to set your DATABASE_URL, REDIS_URL, and MIDTRANS credentials
   ```

4. **Run the Service**:
   ```bash
   go run cmd/server/main.go
   ```

   The service will start listening on `http://localhost:8060`.

---

## 📡 API Endpoints & Documentation

Interactive API documentation is accessible when `ENABLE_DOCS=true`:
- **Scalar UI**: [http://localhost:8060/docs](http://localhost:8060/docs)
- **Swagger UI**: [http://localhost:8060/swagger](http://localhost:8060/swagger)
- **OpenAPI 3.1 Specs**: [http://localhost:8060/docs/openapi.json](http://localhost:8060/docs/openapi.json) or `/docs/openapi.yaml`

### Endpoint Catalog

| Group | Method | Path | Auth / Headers | Description |
| :--- | :--- | :--- | :--- | :--- |
| **Health** | `GET` | `/health` | None | Service & database liveness probe |
| **Customer** | `POST` | `/api/v1/orders` | `X-User-Id` | Checkout and generate Snap token |
| **Customer** | `GET` | `/api/v1/orders` | `X-User-Id` | List paginated customer orders |
| **Customer** | `GET` | `/api/v1/orders/{id}` | `X-User-Id` | Get order detail |
| **Customer** | `POST` | `/api/v1/orders/{id}/cancel` | `X-User-Id` | Cancel pending order |
| **Webhook** | `POST` | `/api/v1/orders/webhook/midtrans` | Public (SHA-512) | Midtrans payment notification webhook |
| **Admin** | `GET` | `/api/v1/admin/orders` | `X-User-Role: admin` | List all orders with filters |
| **Admin** | `PATCH` | `/api/v1/admin/orders/{id}/status`| `X-User-Role: admin` | Update order status (Shipped, Completed, etc.) |
| **Dev Simulation** | `POST` | `/api/v1/dev/orders/{id}/simulate-success`| `DEV=true` | Force order status to `PAID` |
| **Dev Simulation** | `POST` | `/api/v1/dev/orders/{id}/simulate-cancel` | `DEV=true` | Force order status to `CANCELLED` |
| **Dev Simulation** | `POST` | `/api/v1/dev/orders/{id}/simulate-expire` | `DEV=true` | Force order status to `EXPIRED` |

---

## 📦 Transactional Outbox & Kafka Pipeline

To guarantee consistency between database records and emitted events, every state transition generates an `OutboxEvent` record within the same DB transaction.

The background **Outbox Worker** (`internal/outbox/worker.go`):
1. Polls for `PENDING` outbox records every `200ms`.
2. Publishes JSON events to the designated Kafka topic.
3. Updates the outbox record to `PUBLISHED` (or increments `retryCount` on failure).

### Kafka Topics & Event Schemas

#### Outbound Domain Events (`order.events`)

| Event Type | Trigger Condition | Payload Summary | Downstream Consumers |
| :--- | :--- | :--- | :--- |
| `order.created` | Order initiated | Order ID, User ID, Items, Total Amount, Snap Redirect URL | `store_notification` |
| `order.paid` | Payment settlement confirmed | Order ID, Payment Type, Transaction ID, Paid Timestamp, Line Items | `store_notification` |
| `order.cancelled` | Order cancelled by user/admin/ban/delete | Order ID, Cancellation Reason, User ID, Line Items | `store_product` (Restock inventory), `store_notification` |
| `order.expired` | Payment window expired | Order ID, Expiration Reason, Line Items | `store_product` (Restock inventory), `store_notification` |
| `order.fulfilled` | Order completed / delivered | Order ID, Courier Info, Tracking Number | `store_notification` |

#### Inbound Consumer Events (`user.events`)

| Event Type | Source Service | Action in `store_order` | PII Impact |
| :--- | :--- | :--- | :--- |
| `user.deleted` | `store_user` | Cancels unpaid orders, emits `order.cancelled` outbox event for inventory restock | **Anonymized**: `userEmail` masked, `shippingAddress` set to `[ANONYMIZED]`, tokens cleared |
| `user.banned` | `store_user` | Cancels unpaid orders, emits `order.cancelled` outbox event for inventory restock | **Preserved**: All customer emails, shipping addresses, and completed orders retained for audit |

---

## 🛡 Redis Rate Limiting Rules

The service implements sliding-window counter rate limiting with the following policy definitions:

| Scope / Route | Limit | Window | Key Strategy |
| :--- | :--- | :--- | :--- |
| **Global IP Ingress** | 120 req | 1 minute | Client IP |
| **Midtrans Webhook** | 300 req | 1 minute | `webhook:ip:<ip>` |
| **Checkout (`POST /api/v1/orders`)** | 3 req | 10 seconds | `checkout:user:<userId>` |
| **Cancel (`POST /api/v1/orders/{id}/cancel`)** | 5 req | 1 minute | `cancel:user:<userId>` |

### Resilience & Degraded Headers
- **Fail-Open Policy**: If Redis is unreachable or latency exceeds `25ms`, the limiter allows traffic to pass uninterrupted while attaching `X-RateLimit-Degraded: true`.
- **Response Headers**:
  - `X-RateLimit-Limit`: Maximum requests permitted within the window.
  - `X-RateLimit-Remaining`: Remaining request quota.
  - `Retry-After`: Seconds until quota replenishment (on `429 Too Many Requests`).

---

## 🧪 Developer Simulation Mode

When developing without active Midtrans webhooks, set `DEV=true` in `.env`.

### Simulate Order Payment
```bash
curl -X POST http://localhost:8060/api/v1/dev/orders/<ORDER_ID>/simulate-success
```

### Simulate Order Cancellation
```bash
curl -X POST http://localhost:8060/api/v1/dev/orders/<ORDER_ID>/simulate-cancel
```

### Simulate Order Expiration
```bash
curl -X POST http://localhost:8060/api/v1/dev/orders/<ORDER_ID>/simulate-expire
```

---

## 🧪 Testing

Execute unit and integration tests:

```bash
# Run all test packages
go test -v ./...

# Run test suite with race detector and coverage
go test -race -cover ./...
```

---

## 🐳 Docker Deployment

A production-ready, multi-stage Docker build is provided:

1. **Build Container Image**:
   ```bash
   docker build -t store_order:latest .
   ```

2. **Run Container**:
   ```bash
   docker run -d \
     --name store_order \
     -p 8060:8060 \
     -e DATABASE_URL="postgresql://postgres:password@postgres:5432/store_order?schema=public" \
     -e REDIS_URL="redis:6379" \
     -e KAFKA_BROKERS="kafka:9092" \
     -e PRODUCT_SERVICE_URL="http://product_service:8040" \
     -e MIDTRANS_SERVER_KEY="SB-Mid-server-key" \
     store_order:latest
   ```

3. **Check Container Health**:
   ```bash
   docker inspect --format='{{json .State.Health}}' store_order
   ```
