package notification

import (
	"context"
	"testing"

	domain "payment_service/internal/domain/notification"
)

func TestLogNotifierSend(t *testing.T) {
	n := domain.NewNotification("id-1", "pay-1", "payment.completed", domain.ChannelEmail, "user@example.com", "olá")
	if err := NewLogNotifier().Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
