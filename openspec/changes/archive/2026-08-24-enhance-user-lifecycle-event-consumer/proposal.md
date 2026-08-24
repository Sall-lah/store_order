## Why

The `store_user` service publishes account lifecycle domain events (`user.deleted` and `user.banned`) using flat JSON payloads with dual-key aliases (`event` / `event_type`, `userId` / `user_id`, and `reason`). While `store_order` currently processes both events successfully, its consumer payload parser extracts the optional `reason` attribute exclusively from nested `data.reason` objects and resolves event type only from `event_type` or `type`. Consequently, administrative ban reasons and custom event envelope keyings from `store_user` are lost during extraction, falling back to default strings.

## What Changes

- Update `UserEventEnvelope` in `internal/consumer/user_event_consumer.go` to declare top-level `Event` (`json:"event"`) and `Reason` (`json:"reason"`) fields.
- Enhance `ExtractUserLifecycleEvent` to check `env.Event` when resolving event type and fall back to top-level `env.Reason` when `reason` is not present in nested `data`.
- Add unit test coverage in `internal/consumer/user_event_consumer_test.go` verifying top-level `reason` extraction, top-level `event` key resolution, and flat payload compatibility with `store_user`.

## Capabilities

### New Capabilities
<!-- None -->

### Modified Capabilities
- `user-lifecycle-events`: Enhanced payload extraction supporting flat top-level `reason` and `event` envelope keys alongside existing nested schema variants.

## Impact

- **Affected Code**: `internal/consumer/user_event_consumer.go`, `internal/consumer/user_event_consumer_test.go`.
- **APIs/Protocols**: Kafka consumer on `user.events` topic.
- **Breaking Changes**: None. Fully backward-compatible with nested payloads and flat payloads.
