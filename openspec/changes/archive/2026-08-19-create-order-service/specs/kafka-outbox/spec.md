## ADDED Requirements

### Requirement: Transactional Outbox Event Persistence
The system SHALL persist all outbound domain events (`order.created`, `order.paid`, `order.cancelled`, `order.expired`, `order.fulfilled`) into the PostgreSQL `outbox_events` table within the same ACID database transaction that modifies the order state.

#### Scenario: Atomic event capture on state change
- **WHEN** an order state transition occurs (such as checkout, payment settlement, or cancellation)
- **THEN** the order record update and the new outbox event record are committed together in a single atomic database transaction

#### Scenario: Rollback on failure
- **WHEN** an error occurs during order processing or database write
- **THEN** both the order update and the outbox event insertion are rolled back completely, preventing orphaned events

### Requirement: Background Outbox Publisher Worker
The system SHALL run a background Go worker goroutine utilizing `segmentio/kafka-go` that periodically fetches pending events from the outbox table and publishes them to the designated Kafka topics.

#### Scenario: Publishing pending events
- **WHEN** pending events exist in the `outbox_events` table
- **THEN** the outbox worker reads a batch of pending events, publishes them to Kafka with the order ID as partition key, and marks the records as `PUBLISHED` with `published_at` timestamp

#### Scenario: Handling Kafka broker failure
- **WHEN** the Kafka broker is temporarily unreachable during message publishing
- **THEN** the outbox worker increments `retry_count`, logs the error with exponential backoff, and keeps the records as `PENDING` for redelivery on the next interval

#### Scenario: Graceful worker shutdown
- **WHEN** the service receives a termination signal (`SIGINT` / `SIGTERM`)
- **THEN** the outbox worker flushes active publishing batches and closes the Kafka writer cleanly before exiting
