## Context

`store_order` is an event-driven microservice in Go providing order lifecycle management, transactional outbox publishing to Kafka, and Midtrans payment processing over HTTP (Chi router). Upstream services such as `store_user` need a synchronous, low-latency inter-service RPC to determine whether a user can safely be deleted or if active in-flight orders exist that block deletion.

The protobuf schema in `github.com/Sall-lah/store_proto/gen/go/store/order/v1` defines the `OrderService.CheckActiveOrders` contract.

## Goals / Non-Goals

**Goals:**
- Implement `OrderServiceServer` conforming to `github.com/Sall-lah/store_proto/gen/go/store/order/v1`.
- Expose a dedicated gRPC server concurrently alongside the Chi HTTP server in `cmd/server/main.go`.
- Query the database via Prisma client to identify active in-flight orders (`PENDING_PAYMENT`, `PAID`, `PROCESSING`, `SHIPPED`).
- Provide clean mapping between Prisma `db.OrderStatus` and protobuf `orderv1.OrderStatus`.
- Integrate gRPC server into the application graceful shutdown lifecycle.

**Non-Goals:**
- Changing existing HTTP endpoints or altering Prisma schema tables.
- Implementing gRPC clients to other services within `store_order` (this is server-side only).
- Exposing customer-facing checkout endpoints over gRPC (checkout remains REST/HTTP for API Gateway and Midtrans webhooks).

## Decisions

### 1. Dedicated gRPC Port vs Port Multiplexing (cmux)
- **Decision**: Run gRPC on a distinct port configured via `GRPC_PORT` (default `:50051`).
- **Rationale**: Separate listener sockets provide simpler container port mappings, clear Prometheus/metrics isolation, and prevent HTTP/2 framing collisions with HTTP/1.1 endpoints.
- **Alternatives Considered**: `cmux` connection multiplexing on a single port was rejected due to unnecessary connection wrapping complexity and potential latency overhead.

### 2. Code Organization & Modular Layering
- **Decision**: Create a dedicated package `internal/grpc` containing:
  - `server.go`: gRPC server initialization, interceptors (recovery, logging), and lifecycle hooks (`Start`, `Stop`).
  - `order_service.go`: Implements `orderv1.OrderServiceServer` and handles `CheckActiveOrders`.
  - `mapper.go`: Bidirectional mapping between Prisma enum types and Protobuf enums/messages.
- **Rationale**: Keeps gRPC transport logic strictly separated from HTTP handlers in `internal/handler` while reusing the existing `OrderRepository` data layer.

### 3. Repository Method for Active Order Filtering
- **Decision**: Add `ListActiveOrdersByUserID(ctx context.Context, userID string) ([]db.OrderModel, error)` to `OrderRepository`.
- **Rationale**: Direct database query filtering on `userId = ?` AND `status IN ('PENDING_PAYMENT', 'PAID', 'PROCESSING', 'SHIPPED')` ensures minimal memory consumption and avoids pulling completed/cancelled historical records into memory.

### 4. Status Mapping
- **Decision**: Explicit mapping function translating DB status strings to `orderv1.OrderStatus`:
  - `db.OrderStatusPendingPayment` → `orderv1.OrderStatus_ORDER_STATUS_PENDING_PAYMENT`
  - `db.OrderStatusPaid` → `orderv1.OrderStatus_ORDER_STATUS_PAID`
  - `db.OrderStatusProcessing` → `orderv1.OrderStatus_ORDER_STATUS_PROCESSING`
  - `db.OrderStatusShipped` → `orderv1.OrderStatus_ORDER_STATUS_SHIPPED`
  - `db.OrderStatusCompleted` → `orderv1.OrderStatus_ORDER_STATUS_COMPLETED`
  - `db.OrderStatusCancelled` → `orderv1.OrderStatus_ORDER_STATUS_CANCELLED`
  - `db.OrderStatusExpired` → `orderv1.OrderStatus_ORDER_STATUS_EXPIRED`
  - Any unknown value → `orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED`

## Risks / Trade-offs

- **[Dependency Resolution for `store_proto`]** → In development environments where `store_proto` is a private repository or sibling folder, `go.mod` can use a `replace github.com/Sall-lah/store_proto => ../store_proto` or standard `GOPRIVATE` token configuration.
- **[Shutdown Coordination]** → If shutdown timeout is too short, long-running RPCs might get aborted. We will invoke `grpcServer.GracefulStop()` concurrently with HTTP server shutdown within a bounded 10s context.
- **[Database Load]** → Active order checks execute indexed queries on `userId` and `status` (`@@index([userId])`, `@@index([status])`), ensuring sub-millisecond query execution.
