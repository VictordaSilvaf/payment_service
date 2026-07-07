package notification

import "testing"

func TestNotificationLifecycle(t *testing.T) {
	n := NewNotification("id-1", "pay-1", "payment.completed", ChannelEmail, "user@example.com", "olá")
	if n.Status != StatusPending {
		t.Fatalf("expected pending, got %s", n.Status)
	}
	if n.PaymentID != "pay-1" || n.Channel != ChannelEmail {
		t.Fatalf("unexpected fields: %+v", n)
	}

	n.MarkFailed("smtp down")
	if n.Status != StatusFailed || n.LastError != "smtp down" {
		t.Fatalf("unexpected state after fail: %+v", n)
	}

	n.MarkSent()
	if n.Status != StatusSent {
		t.Fatalf("expected sent, got %s", n.Status)
	}
	if n.LastError != "" {
		t.Fatalf("expected cleared error, got %q", n.LastError)
	}
}
