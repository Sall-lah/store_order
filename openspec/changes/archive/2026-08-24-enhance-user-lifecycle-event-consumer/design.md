## Context

The `store_user` microservice publishes account lifecycle events (`user.deleted` and `user.banned`) using flat JSON payloads containing dual-compatible keys (`event`/`event_type`, `userId`/`user_id`, and `reason`). 

In `store_order`, `UserEventEnvelope` currently binds `event_type`, `type`, `user_id`, `userId`, and `id`, while `reason` is parsed only if present inside a nested `data` object (`env.Data`). When `store_user` dispatches a flat `user.banned` event with a top-level `reason`, the ban reason is dropped during extraction, causing `OrderService.HandleUserBanned` to fall back to the default cancellation message (`"User account banned"`).

## Goals / Non-Goals

**Goals:**
- Extend `UserEventEnvelope` with top-level `Event` (`json:"event"`) and `Reason` (`json:"reason"`).
- Update `ExtractUserLifecycleEvent` to resolve event type from `env.Event` if `env.EventType` and `env.Type` are absent.
- Update `ExtractUserLifecycleEvent` to fall back to top-level `env.Reason` when `reason` is not provided within a nested `data` object.
- Preserve 100% backward compatibility for nested payloads and legacy callers of `ExtractUserDeletedID`.
- Add unit tests covering flat payloads with top-level `reason` and `event` attributes.

**Non-Goals:**
- Modifying `OrderService.HandleUserBanned` or `OrderService.HandleUserDeleted` domain logic (already working properly).
- Modifying database schemas or outbox table structures.

## Decisions

### 1. Dual-Source Reason & Event Resolution in Consumer Extraction
- **Choice**: Extract `reason` from nested `data.reason` first, falling back to top-level `env.Reason`. Extract `eventType` checking `env.EventType`, `env.Type`, and `env.Event`.
- **Rationale**: Provides symmetric resilience for both flat payloads (dispatched directly by `store_user`) and envelope-wrapped payloads (dispatched by third-party event brokers or gateways).
- **Alternatives Considered**: Modifying `store_user` to wrap payloads in `data: {}` (rejected: breaks existing consumers like `store_auth` that rely on flat keys).

## Risks / Trade-offs

- **[Risk] Whitespace or empty reason strings** → *Mitigation*: Trim whitespace; if empty, `OrderService.HandleUserBanned` cleanly defaults to `"User account banned"`.
- **[Risk] Multiple conflicting event type fields** → *Mitigation*: Maintain deterministic evaluation precedence: `event_type` -> `type` -> `event`.
