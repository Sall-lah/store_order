## Why

When other microservices (such as `store_user`) perform sensitive lifecycle actions like account deletion, they require a synchronous, high-performance check to verify that the user has no in-flight or active orders (`PENDING_PAYMENT`, `PAID`, `PROCESSING`, `SHIPPED`). Without this pre-flight verification, deleting an account could lead to orphaned orders, reconciliation gaps, and broken customer fulfillment.

The gRPC contract defined in `github.com/Sall-lah/store_proto` (`OrderService.CheckActiveOrders`) establishes the protocol for this check. `store_order` needs to implement and serve this gRPC endpoint alongside its existing HTTP server.

## What Changes

- Add gRPC dependency and `github.com/Sall-lah/store_proto` generated Go stubs to `store_order`.
- Introduce configuration support for `GRPC_PORT` (default `:50051`).
- Add gRPC server lifecycle management with graceful shutdown running alongside the existing HTTP server in `cmd/server/main.go`.
- Implement `OrderServiceServer` with `CheckActiveOrders` RPC handler in `internal/grpc` (or `internal/handler/grpc`).
- Implement repository query method (`FindActiveOrdersByUserID`) in `internal/repository/order_repository.go` to fetch in-flight orders with status in `PENDING_PAYMENT`, `PAID`, `PROCESSING`, or `SHIPPED`.
- Add unit tests for gRPC handler and repository active order lookup logic.

## Capabilities

### New Capabilities
- `grpc-active-orders-check`: Exposes the `OrderService.CheckActiveOrders` gRPC RPC endpoint to evaluate whether a user has active or in-flight orders blocking account deletion.

### Modified Capabilities
<!-- No requirement changes to existing HTTP specs -->

## Impact

- **gRPC Server**: A new gRPC server starts listening on configured `GRPC_PORT` (e.g., `:50051`), supporting concurrent requests and graceful shutdown on SIGINT/SIGTERM.
- **Data Layer**: New query in `OrderRepository` to fetch active orders for a given user ID.
- **Dependencies**: Adds `google.golang.org/grpc`, `google.golang.org/protobuf`, and `github.com/Sall-lah/store_proto`.
- **Inter-service Integration**: Enables `store_user` to synchronously check active order status over gRPC before deleting a user account.
