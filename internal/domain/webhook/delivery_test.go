package webhook

import "testing"

func TestDeliveryLifecycle(t *testing.T) {
	d := NewDelivery("sub-1", "evt-1")
	if d.Status != DeliveryPending {
		t.Fatalf("expected pending, got %s", d.Status)
	}
	if d.ID == "" {
		t.Fatal("expected generated id")
	}

	d.Fail("boom")
	if d.Status != DeliveryFailed {
		t.Fatalf("expected failed, got %s", d.Status)
	}
	if d.Attempts != 1 || d.LastError != "boom" {
		t.Fatalf("unexpected state after fail: %+v", d)
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
}
