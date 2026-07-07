package webhook

import (
	"context"
	"testing"

	"payment_service/internal/application/dto"
	"payment_service/internal/infrastructure/persistence/memory"
)

func TestCreateAndListSubscriptions(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWebhookSubscriptionRepository()

	create := NewCreateSubscription(repo)
	list := NewListSubscriptions(repo)

	t.Run("create valid", func(t *testing.T) {
		res, err := create.Execute(ctx, dto.CreateWebhookRequest{
			URL:       "https://merchant.test/hook",
			EventType: "payment.completed",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID == "" || res.Secret == "" {
			t.Fatalf("expected id and generated secret, got %+v", res)
		}
	})

	t.Run("create invalid url", func(t *testing.T) {
		_, err := create.Execute(ctx, dto.CreateWebhookRequest{
			URL:       "not-a-url",
			EventType: "payment.completed",
		})
		if err == nil {
			t.Fatal("expected error for invalid url")
		}
	})

	t.Run("list returns created", func(t *testing.T) {
		res, err := list.Execute(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 subscription, got %d", len(res))
		}
	})
}
