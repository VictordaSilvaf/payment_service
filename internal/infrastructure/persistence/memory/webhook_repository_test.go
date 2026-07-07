package memory

import (
	"context"
	"testing"
	"time"

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
	subs := NewWebhookSubscriptionRepository()
	repo := NewWebhookDeliveryRepository(subs)

	d := webhook.NewDelivery("sub-1", "evt-1", "payment.completed", []byte(`{"id":"p"}`))
	d.MarkDelivered()
	if err := repo.Save(ctx, d); err != nil {
		t.Fatal(err)
	}

	all := repo.All()
	if len(all) != 1 || all[0].Status != webhook.DeliveryDelivered {
		t.Fatalf("expected 1 delivered, got %+v", all)
	}
}

func TestWebhookDeliveryRepositoryFetchRetriable(t *testing.T) {
	ctx := context.Background()
	subs := NewWebhookSubscriptionRepository()
	sub, _ := webhook.NewSubscription("https://a.test/h", "secret", "payment.completed")
	_ = subs.Save(ctx, sub)

	repo := NewWebhookDeliveryRepository(subs)

	// Entrega falha, já no prazo → elegível.
	due := webhook.NewDelivery(sub.ID, "evt-due", "payment.completed", []byte(`{}`))
	due.MarkForRetry("boom", time.Now().Add(-time.Minute))
	_ = repo.Save(ctx, due)

	// Entrega falha, ainda no futuro → não elegível.
	future := webhook.NewDelivery(sub.ID, "evt-future", "payment.completed", []byte(`{}`))
	future.MarkForRetry("boom", time.Now().Add(time.Hour))
	_ = repo.Save(ctx, future)

	// Entrega já entregue → não elegível.
	done := webhook.NewDelivery(sub.ID, "evt-done", "payment.completed", []byte(`{}`))
	done.MarkDelivered()
	_ = repo.Save(ctx, done)

	items, err := repo.FetchRetriable(ctx, 10, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Delivery.EventID != "evt-due" {
		t.Fatalf("expected only evt-due, got %+v", items)
	}
	if items[0].URL != sub.URL || items[0].Secret != sub.Secret {
		t.Fatalf("expected joined subscription data, got url=%s secret=%s", items[0].URL, items[0].Secret)
	}
}
