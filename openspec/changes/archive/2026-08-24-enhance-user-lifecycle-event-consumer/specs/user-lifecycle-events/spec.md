## MODIFIED Requirements

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
