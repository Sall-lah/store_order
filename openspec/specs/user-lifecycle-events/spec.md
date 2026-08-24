# User Lifecycle Events Specification

## Purpose
Enables asynchronous consumption and handling of user lifecycle domain events (`user.deleted` and `user.banned`) emitted from upstream identity services (`store_user`) over Kafka, orchestrating automated unpaid order cleanup, stock reservation release, GDPR PII anonymization for account deletions, and audit retention for account bans.

## Requirements

### Requirement: Consume User Deleted Event
The system SHALL operate a background Kafka consumer group worker subscribed to the user events topic (`user.events`), listening for `user.deleted` events containing the target `user_id`.

#### Scenario: Successfully consume and process user deleted event
- **WHEN** a valid `user.deleted` event is received with a non-empty `user_id`
- **THEN** the consumer initiates account cleanup in the order domain, auto-cancelling any unpaid orders, emitting cancellation outbox events, and anonymizing personal customer data across all associated orders

#### Scenario: Malformed event payload received
- **WHEN** an unparseable or schema-invalid message is received on the topic
- **THEN** the consumer logs an error, rejects/skips the invalid message without panicking, and commits offset to prevent blocking the consumer partition

#### Scenario: Idempotent processing of duplicate user deleted events
- **WHEN** a duplicate `user.deleted` event for an already anonymized user is received
- **THEN** the system executes the cleanup idempotently as a no-op and acknowledges the event without error

### Requirement: Consume User Banned Event
The system SHALL consume `user.banned` domain events from the user events topic (`user.events`), extracting the target `user_id` and optional ban reason (from nested or flat top-level payload attributes), and initiating unpaid order cancellation without modifying or redacting customer PII.

#### Scenario: Successfully consume and process user banned event
- **WHEN** a valid `user.banned` event is received with a non-empty `user_id` and an optional ban reason in either flat top-level or nested data schema
- **THEN** the consumer extracts the user ID, event type, and ban reason, initiating ban cleanup in the order service, cancelling any in-flight unpaid orders and emitting cancellation outbox events while retaining all customer personal data

#### Scenario: Idempotent processing of duplicate user banned events
- **WHEN** a duplicate `user.banned` event is received for a user whose unpaid orders were already cancelled
- **THEN** the system executes the cleanup idempotently as a no-op and acknowledges the event offset without error

#### Scenario: Non-banned and non-deleted events on user topic
- **WHEN** an event with a different lifecycle event type (e.g. `user.created`, `user.updated`) is received
- **THEN** the consumer skips the event and acknowledges offset without triggering cancellation workflows

### Requirement: Graceful Consumer Lifecycle Management
The system SHALL boot the Kafka consumer worker concurrently during service startup and cleanly terminate its connection and message processing loop upon receiving system termination signals (`SIGINT`, `SIGTERM`).

#### Scenario: Service startup boots consumer
- **WHEN** the application starts up
- **THEN** the user lifecycle Kafka consumer connects to configured brokers and starts polling messages from `user.events` topic

#### Scenario: Service shutdown stops consumer gracefully
- **WHEN** the application receives a shutdown signal
- **THEN** the consumer ceases message polling, allows the active message processing handler to finish, closes reader connections, and exits
