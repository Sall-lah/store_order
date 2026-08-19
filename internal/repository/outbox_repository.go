package repository

import (
	"context"
	"time"

	"github.com/Sall-lah/store_order/internal/db"
)

// OutboxRepository defines the persistence contract for reading and acknowledging transactional outbox events.
type OutboxRepository interface {
	GetPendingEvents(ctx context.Context, limit int) ([]db.OutboxEventModel, error)
	MarkEventPublished(ctx context.Context, eventID string) error
	IncrementEventRetry(ctx context.Context, eventID string, errorMsg string) error
}

// SQLOutboxRepository implements OutboxRepository using Prisma Client Go.
type SQLOutboxRepository struct {
	client *db.PrismaClient
}

// NewOutboxRepository constructs an SQLOutboxRepository instance.
// Why: Decouples outbox table querying and state transitions from worker implementations.
func NewOutboxRepository(client *db.PrismaClient) *SQLOutboxRepository {
	return &SQLOutboxRepository{client: client}
}

// GetPendingEvents fetches a batch of un-published outbox events ordered chronologically.
// Why: Provides the publisher worker with a deterministic, bounded batch of pending domain messages.
func (r *SQLOutboxRepository) GetPendingEvents(ctx context.Context, limit int) ([]db.OutboxEventModel, error) {
	if limit <= 0 {
		limit = 50
	}

	return r.client.OutboxEvent.FindMany(
		db.OutboxEvent.Status.Equals(db.OutboxStatusPending),
	).OrderBy(
		db.OutboxEvent.CreatedAt.Order(db.SortOrderAsc),
	).Take(limit).Exec(ctx)
}

// MarkEventPublished updates an outbox event status to PUBLISHED and stamps the current time.
// Why: Confirms broker message acknowledgment to prevent redelivery of processed events.
func (r *SQLOutboxRepository) MarkEventPublished(ctx context.Context, eventID string) error {
	now := time.Now()
	_, err := r.client.OutboxEvent.FindUnique(
		db.OutboxEvent.ID.Equals(eventID),
	).Update(
		db.OutboxEvent.Status.Set(db.OutboxStatusPublished),
		db.OutboxEvent.PublishedAt.Set(now),
	).Exec(ctx)
	return err
}

// IncrementEventRetry records a publishing error and increments the retry count.
// Why: Tracks publication failure telemetry for debugging and backoff without dropping the event.
func (r *SQLOutboxRepository) IncrementEventRetry(ctx context.Context, eventID string, errorMsg string) error {
	event, err := r.client.OutboxEvent.FindUnique(
		db.OutboxEvent.ID.Equals(eventID),
	).Exec(ctx)
	if err != nil {
		return err
	}

	retries := event.RetryCount + 1
	status := db.OutboxStatusPending
	if retries > 10 {
		status = db.OutboxStatusFailed
	}

	_, err = r.client.OutboxEvent.FindUnique(
		db.OutboxEvent.ID.Equals(eventID),
	).Update(
		db.OutboxEvent.RetryCount.Set(retries),
		db.OutboxEvent.ErrorMessage.Set(errorMsg),
		db.OutboxEvent.Status.Set(status),
	).Exec(ctx)
	return err
}
