package memory

import (
	"context"
	"encoding/json"
	"testing"

	"payment_service/internal/domain/outbox"
)

func TestOutboxRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewOutboxRepository()

	e1 := outbox.NewEvent("agg-1", "payment.created", json.RawMessage(`{"n":1}`))
	e2 := outbox.NewEvent("agg-2", "payment.created", json.RawMessage(`{"n":2}`))
	if err := repo.Add(ctx, e1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(ctx, e2); err != nil {
		t.Fatal(err)
	}

	t.Run("fetch respects limit", func(t *testing.T) {
		events, err := repo.FetchUnpublished(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
	})

	t.Run("mark published removes from pending", func(t *testing.T) {
		if err := repo.MarkPublished(ctx, e1.ID); err != nil {
			t.Fatal(err)
		}

		events, err := repo.FetchUnpublished(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 pending after mark, got %d", len(events))
		}
		if events[0].ID != e2.ID {
			t.Fatalf("expected remaining event to be e2, got %s", events[0].ID)
		}
	})

	t.Run("mark unknown id is noop", func(t *testing.T) {
		if err := repo.MarkPublished(ctx, "missing"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
