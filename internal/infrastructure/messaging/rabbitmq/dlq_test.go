package rabbitmq

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNaming(t *testing.T) {
	if got := dlxName("payment.events"); got != "payment.events.dlx" {
		t.Fatalf("unexpected dlx name: %s", got)
	}
	if got := dlqName("payment.created"); got != "payment.created.dlq" {
		t.Fatalf("unexpected dlq name: %s", got)
	}
}

func TestProcessWithRetrySucceedsFirstTry(t *testing.T) {
	calls := 0
	err := processWithRetry(context.Background(), 3, time.Millisecond, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestProcessWithRetryEventuallySucceeds(t *testing.T) {
	calls := 0
	err := processWithRetry(context.Background(), 3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestProcessWithRetryExhausts(t *testing.T) {
	boom := errors.New("boom")
	calls := 0
	err := processWithRetry(context.Background(), 3, time.Millisecond, func() error {
		calls++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom error, got %v", err)
	}
	// maxRetries=3 → 1 tentativa inicial + 3 retentativas = 4 chamadas.
	if calls != 4 {
		t.Fatalf("expected 4 calls, got %d", calls)
	}
}

func TestProcessWithRetryStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	boom := errors.New("boom")
	calls := 0
	err := processWithRetry(ctx, 5, time.Hour, func() error {
		calls++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom error, got %v", err)
	}
	// Contexto cancelado interrompe antes de esperar o delay: só a 1ª chamada roda.
	if calls != 1 {
		t.Fatalf("expected 1 call before cancel, got %d", calls)
	}
}
