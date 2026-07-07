package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	domainoutbox "payment_service/internal/domain/outbox"
	"payment_service/internal/infrastructure/persistence/memory"
)

type mockPublisher struct {
	mu        sync.Mutex
	published []published
	err       error
}

type published struct {
	routingKey string
	body       []byte
}

func (m *mockPublisher) Publish(_ context.Context, routingKey string, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.published = append(m.published, published{routingKey: routingKey, body: body})
	return nil
}

func (m *mockPublisher) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.published)
}

func seedEvents(t *testing.T, repo *memory.OutboxRepository, n int) {
	t.Helper()
	for range n {
		e := domainoutbox.NewEvent("agg", "payment.created", json.RawMessage(`{"id":"x"}`))
		if err := repo.Add(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRelayDispatchPublishesAndMarks(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewOutboxRepository()
	seedEvents(t, repo, 3)

	pub := &mockPublisher{}
	relay := NewRelay(repo, pub, time.Second, 10)

	if err := relay.dispatch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pub.count() != 3 {
		t.Fatalf("expected 3 published, got %d", pub.count())
	}

	pending, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after dispatch, got %d", len(pending))
	}
}

func TestRelayDispatchStopsOnPublishError(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewOutboxRepository()
	seedEvents(t, repo, 2)

	pub := &mockPublisher{err: errors.New("broker down")}
	relay := NewRelay(repo, pub, time.Second, 10)

	err := relay.dispatch(ctx)
	if err == nil {
		t.Fatal("expected error when publish fails")
	}

	// Nada foi marcado como publicado: eventos continuam pendentes p/ retry.
	pending, ferr := repo.FetchUnpublished(ctx, 10)
	if ferr != nil {
		t.Fatal(ferr)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 still pending, got %d", len(pending))
	}
}

func TestRelayDispatchEmpty(t *testing.T) {
	repo := memory.NewOutboxRepository()
	relay := NewRelay(repo, &mockPublisher{}, time.Second, 10)

	if err := relay.dispatch(context.Background()); err != nil {
		t.Fatalf("unexpected error on empty outbox: %v", err)
	}
}

func TestRelayRunStopsOnContextCancel(t *testing.T) {
	repo := memory.NewOutboxRepository()
	seedEvents(t, repo, 1)
	pub := &mockPublisher{}
	relay := NewRelay(repo, pub, 5*time.Millisecond, 10)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()

	// Dá tempo de pelo menos um tick processar o evento pendente.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("relay did not stop after context cancel")
	}

	if pub.count() == 0 {
		t.Fatal("expected at least one event published before cancel")
	}
}
