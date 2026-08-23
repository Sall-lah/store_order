# gRPC Active Orders Check Specification

## Purpose
Exposes high-performance gRPC RPCs for inter-service communication conforming to `store_proto` contract (`github.com/Sall-lah/store_proto/gen/go/store/order/v1`), enabling upstream services (such as `store_user`) to perform synchronous pre-flight checks before executing account lifecycle transitions.

## Requirements

### Requirement: Check Active Orders gRPC RPC
The system SHALL expose the `CheckActiveOrders` RPC endpoint on the `OrderService` gRPC service according to the protobuf schema defined in `store_proto` (`github.com/Sall-lah/store_proto/gen/go/store/order/v1`). The endpoint SHALL inspect the database for orders belonging to the requested `user_id` that are in active lifecycle states (`PENDING_PAYMENT`, `PAID`, `PROCESSING`, `SHIPPED`), and return whether deletion is blocked along with active order counts and summary metadata.

#### Scenario: User has active orders blocking account deletion
- **WHEN** a client invokes `CheckActiveOrders` with a valid `user_id` who has orders in `PENDING_PAYMENT`, `PAID`, `PROCESSING`, or `SHIPPED` status
- **THEN** the system responds with `has_active_orders: true`, `active_order_count` equal to the number of active orders, an `active_orders` array containing lightweight summary metadata (`order_id`, `order_number`, protobuf `status`, `total_amount`, `created_at` ISO 8601 string) for each active order, and an explanatory human-readable message

#### Scenario: User has only terminal orders
- **WHEN** a client invokes `CheckActiveOrders` with a valid `user_id` who only has orders in `COMPLETED`, `CANCELLED`, or `EXPIRED` status
- **THEN** the system responds with `has_active_orders: false`, `active_order_count: 0`, an empty `active_orders` list, and a message indicating no active orders exist

#### Scenario: User has no order records
- **WHEN** a client invokes `CheckActiveOrders` with a `user_id` that does not exist in the orders database
- **THEN** the system responds with `has_active_orders: false`, `active_order_count: 0`, an empty `active_orders` list, and a message indicating no active orders exist

#### Scenario: Request contains missing or blank user ID
- **WHEN** a client invokes `CheckActiveOrders` with an empty or whitespace-only `user_id`
- **THEN** the system rejects the call with gRPC status code `InvalidArgument` (3) and a descriptive error message

### Requirement: gRPC Server Runtime and Configuration
The system SHALL provide gRPC server configuration via environment variable `GRPC_PORT` (defaulting to `50051`), boot the gRPC server concurrently alongside the existing Chi HTTP server and Outbox worker on service startup, and execute a graceful shutdown (`GracefulStop`) upon receiving system termination signals (`SIGINT`, `SIGTERM`).

#### Scenario: gRPC server binds to configured port
- **WHEN** the application starts with `GRPC_PORT=50051`
- **THEN** the gRPC server binds and begins accepting incoming RPC connections on port 50051 concurrently with HTTP traffic on `PORT`

#### Scenario: Graceful shutdown on signal receipt
- **WHEN** the application receives a `SIGINT` or `SIGTERM` signal
- **THEN** the gRPC server calls `GracefulStop()`, waiting for active RPCs to finish within the shutdown timeout period before releasing socket resources
