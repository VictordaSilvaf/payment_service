package memory

import (
	"context"
	"testing"

	"payment_service/internal/domain/notification"
)

func TestNotificationRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewNotificationRepository()

	n := notification.NewNotification("id-1", "pay-1", "payment.completed", notification.ChannelEmail, "user@example.com", "olá")
	n.MarkSent()
	if err := repo.Save(ctx, n); err != nil {
		t.Fatal(err)
	}

	// Upsert: salvar de novo com o mesmo id não duplica.
	if err := repo.Save(ctx, n); err != nil {
		t.Fatal(err)
	}

	all := repo.All()
	if len(all) != 1 || all[0].Status != notification.StatusSent {
		t.Fatalf("expected 1 sent notification, got %+v", all)
	}
}
