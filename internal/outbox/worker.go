package outbox

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Sall-lah/store_order/internal/kafka"
	"github.com/Sall-lah/store_order/internal/repository"
)

// Worker polls pending outbox events from the database and publishes them to Kafka.
type Worker struct {
	outboxRepo   repository.OutboxRepository
	producer     kafka.Producer
	pollInterval time.Duration
	batchSize    int
	stopChan     chan struct{}
	wg           sync.WaitGroup
	isDevMode    bool
}

// NewWorker constructs an outbox background worker with configurable polling cadence.
// Why: Decouples HTTP transaction completion from network message broker delivery, guaranteeing at-least-once delivery.
func NewWorker(
	outboxRepo repository.OutboxRepository,
	producer kafka.Producer,
	pollInterval time.Duration,
	batchSize int,
	isDevMode bool,
) *Worker {
	if pollInterval <= 0 {
		pollInterval = 200 * time.Millisecond
	}
	if batchSize <= 0 {
		batchSize = 50
	}

	return &Worker{
		outboxRepo:   outboxRepo,
		producer:     producer,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		stopChan:     make(chan struct{}),
		isDevMode:    isDevMode,
	}
}

// Start launches the background polling loop in a managed goroutine.
// Why: Allows asynchronous execution alongside HTTP server lifecycle without blocking application startup.
func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
	log.Println("[Outbox Worker] Background publisher worker started.")
}

// Stop signals the worker loop to terminate and blocks until pending in-flight publications complete.
// Why: Ensures clean drain of currently processing message batches during SIGINT/SIGTERM graceful shutdown.
func (w *Worker) Stop() {
	close(w.stopChan)
	w.wg.Wait()
	log.Println("[Outbox Worker] Background publisher worker stopped gracefully.")
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	events, err := w.outboxRepo.GetPendingEvents(ctx, w.batchSize)
	if err != nil {
		log.Printf("[Outbox Worker] Error fetching pending outbox events: %v", err)
		return
	}

	if len(events) == 0 {
		return
	}

	for _, event := range events {
		pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := w.producer.Publish(pubCtx, event.Topic, event.AggregateID, []byte(event.Payload))
		cancel()

		if err != nil {
			log.Printf("[Outbox Worker] Failed to publish event %s to topic %s: %v", event.ID, event.Topic, err)
			_ = w.outboxRepo.IncrementEventRetry(ctx, event.ID, err.Error())
			continue
		}

		if err := w.outboxRepo.MarkEventPublished(ctx, event.ID); err != nil {
			log.Printf("[Outbox Worker] Failed to mark event %s as published: %v", event.ID, err)
		} else {
			log.Printf("[Outbox Worker] Successfully published event %s [%s] to topic %s", event.ID, event.AggregateType, event.Topic)
		}
	}
}
