package webhook

import (
	"testing"
	"time"
)

func TestDeliveryLifecycle(t *testing.T) {
	d := NewDelivery("sub-1", "evt-1", "payment.completed", []byte(`{"id":"p"}`))
	if d.Status != DeliveryPending {
		t.Fatalf("expected pending, got %s", d.Status)
	}
	if d.ID == "" {
		t.Fatal("expected generated id")
	}
	if d.EventType != "payment.completed" || string(d.Payload) != `{"id":"p"}` {
		t.Fatalf("unexpected event data: %+v", d)
	}

	next := time.Now().Add(time.Minute)
	d.MarkForRetry("boom", next)
	if d.Status != DeliveryFailed {
		t.Fatalf("expected failed, got %s", d.Status)
	}
	if d.Attempts != 1 || d.LastError != "boom" {
		t.Fatalf("unexpected state after retry: %+v", d)
	}
	if !d.NextAttemptAt.Equal(next) {
		t.Fatalf("expected next attempt scheduled, got %v", d.NextAttemptAt)
	}

	d.MarkDelivered()
	if d.Status != DeliveryDelivered {
		t.Fatalf("expected delivered, got %s", d.Status)
	}
	if d.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", d.Attempts)
	}
	if d.LastError != "" {
		t.Fatalf("expected cleared error, got %q", d.LastError)
	}
	if !d.NextAttemptAt.IsZero() {
		t.Fatal("expected next attempt cleared after delivery")
	}
}

func TestDeliveryMarkExhausted(t *testing.T) {
	d := NewDelivery("sub-1", "evt-1", "payment.completed", nil)
	d.MarkExhausted("gave up")
	if d.Status != DeliveryExhausted {
		t.Fatalf("expected exhausted, got %s", d.Status)
	}
	if d.Attempts != 1 || d.LastError != "gave up" {
		t.Fatalf("unexpected state: %+v", d)
	}
}
