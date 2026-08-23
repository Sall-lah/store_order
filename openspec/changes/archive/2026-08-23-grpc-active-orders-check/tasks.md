## 1. Dependencies & Configuration

- [x] 1.1 Add `github.com/Sall-lah/store_proto` and `google.golang.org/grpc` dependencies to `go.mod`
- [x] 1.2 Add `GRPCPort` field to `internal/config/config.go` with default `50051` and update `.env.example`

## 2. Data Access Layer

- [x] 2.1 Add `ListActiveOrdersByUserID(ctx context.Context, userID string) ([]db.OrderModel, error)` to `OrderRepository` in `internal/repository/order_repository.go`

## 3. gRPC Service Implementation

- [x] 3.1 Create `internal/grpc/mapper.go` with status conversion between Prisma `db.OrderStatus` and `orderv1.OrderStatus`, and helper to format `orderv1.ActiveOrderSummary`
- [x] 3.2 Create `internal/grpc/order_service.go` implementing `orderv1.OrderServiceServer` and handling `CheckActiveOrders` with validation and error responses
- [x] 3.3 Create `internal/grpc/server.go` for server initialization, service registration, and graceful start/stop lifecycle methods

## 4. Server Lifecycle Integration

- [x] 4.1 Wire gRPC server into `cmd/server/main.go` to run concurrently alongside HTTP server with graceful shutdown handling

## 5. Testing & Verification

- [x] 5.1 Write unit tests for gRPC handler and status mappers in `internal/grpc/order_service_test.go`
- [x] 5.2 Run `go test ./...` and verify clean build with `go build ./cmd/server`
