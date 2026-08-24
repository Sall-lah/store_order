## 1. Consumer Model & Parser Enhancements

- [x] 1.1 Add `Event` (`json:"event"`) and `Reason` (`json:"reason"`) fields to `UserEventEnvelope` in `internal/consumer/user_event_consumer.go`
- [x] 1.2 Update `ExtractUserLifecycleEvent` in `internal/consumer/user_event_consumer.go` to fall back to `env.Event` for event type resolution and `env.Reason` for top-level ban reasons

## 2. Unit Testing & Verification

- [x] 2.1 Add unit tests in `internal/consumer/user_event_consumer_test.go` covering flat payload `user.banned` with top-level `reason` and `event` keys
- [x] 2.2 Run full test suite with `go test ./...` to verify zero regressions across consumer, handlers, and gRPC services
