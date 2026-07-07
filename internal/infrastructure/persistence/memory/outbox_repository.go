package memory

import (
	"context"
	"sync"
	"time"

	"payment_service/internal/domain/outbox"
)

type OutboxRepository struct {
	mu     sync.Mutex
	events []outbox.Event
}

func NewOutboxRepository() *OutboxRepository {
	return &OutboxRepository{events: make([]outbox.Event, 0)}
}

func (r *OutboxRepository) Add(_ context.Context, event outbox.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *OutboxRepository) FetchUnpublished(_ context.Context, limit int) ([]outbox.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pending := make([]outbox.Event, 0, limit)
	for _, e := range r.events {
		if e.PublishedAt != nil {
			continue
		}
		pending = append(pending, e)
		if len(pending) == limit {
			break
		}
	}
	return pending, nil
}

func (r *OutboxRepository) MarkPublished(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].PublishedAt = &now
			return nil
		}
	}
	return nil
}
