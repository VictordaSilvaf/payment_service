package outbox

import (
	"encoding/json"
	"testing"
)

func TestNewEvent(t *testing.T) {
	payload := json.RawMessage(`{"id":"abc"}`)
	e := NewEvent("agg-1", "payment.created", payload)

	if e.ID == "" {
		t.Fatal("expected generated id")
	}
	if e.AggregateID != "agg-1" {
		t.Fatalf("unexpected aggregate id: %s", e.AggregateID)
	}
	if e.Type != "payment.created" {
		t.Fatalf("unexpected type: %s", e.Type)
	}
	if string(e.Payload) != string(payload) {
		t.Fatalf("unexpected payload: %s", e.Payload)
	}
	if e.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
	if e.PublishedAt != nil {
		t.Fatal("expected published_at to be nil for a new event")
	}
}

func TestNewEventGeneratesUniqueIDs(t *testing.T) {
	a := NewEvent("agg", "t", nil)
	b := NewEvent("agg", "t", nil)
	if a.ID == b.ID {
		t.Fatal("expected unique ids")
	}
}
