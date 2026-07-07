package audit

import "testing"

func TestNewAuditEntry(t *testing.T) {
	payload := []byte(`{"id":"pay-1"}`)
	e := NewAuditEntry("evt-1", AggregatePayment, "pay-1", "payment.completed", payload)

	if e.ID != "evt-1" {
		t.Fatalf("unexpected id: %s", e.ID)
	}
	if e.AggregateType != AggregatePayment || e.AggregateID != "pay-1" {
		t.Fatalf("unexpected aggregate: %+v", e)
	}
	if e.EventType != "payment.completed" {
		t.Fatalf("unexpected event type: %s", e.EventType)
	}
	if string(e.Payload) != string(payload) {
		t.Fatalf("unexpected payload: %s", e.Payload)
	}
	if e.RecordedAt.IsZero() {
		t.Fatal("expected recorded_at to be set")
	}
}
