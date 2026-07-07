package memory

import (
	"context"
	"testing"

	"payment_service/internal/domain/webhook"
)

func TestWebhookSubscriptionRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewWebhookSubscriptionRepository()

	active, _ := webhook.NewSubscription("https://a.test/h", "s", "payment.completed")
	other, _ := webhook.NewSubscription("https://b.test/h", "s", "payment.created")
	inactive, _ := webhook.NewSubscription("https://c.test/h", "s", "payment.completed")
	inactive.Active = false

	for _, s := range []*webhook.Subscription{active, other, inactive} {
		if err := repo.Save(ctx, s); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("find all", func(t *testing.T) {
		all, err := repo.FindAll(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != 3 {
			t.Fatalf("expected 3, got %d", len(all))
		}
	})

	t.Run("find active by event type", func(t *testing.T) {
		got, err := repo.FindActiveByEventType(ctx, "payment.completed")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID != active.ID {
			t.Fatalf("expected only the active completed subscription, got %+v", got)
		}
	})
}

func TestWebhookDeliveryRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewWebhookDeliveryRepository()

	d := webhook.NewDelivery("sub-1", "evt-1")
	d.MarkDelivered()
	if err := repo.Save(ctx, d); err != nil {
		t.Fatal(err)
	}

	all := repo.All()
	if len(all) != 1 || all[0].Status != webhook.DeliveryDelivered {
		t.Fatalf("expected 1 delivered, got %+v", all)
	}
}
