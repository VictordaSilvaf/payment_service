package webhook

import (
	"context"
	"testing"
	"time"

	domain "payment_service/internal/domain/webhook"
	"payment_service/internal/infrastructure/persistence/memory"
)

func TestBackoffPolicy(t *testing.T) {
	p := BackoffPolicy{MaxAttempts: 3, BaseDelay: time.Second}

	if got := p.nextDelay(1); got != time.Second {
		t.Fatalf("attempt 1: expected 1s, got %v", got)
	}
	if got := p.nextDelay(2); got != 2*time.Second {
		t.Fatalf("attempt 2: expected 2s, got %v", got)
	}
	if got := p.nextDelay(3); got != 4*time.Second {
		t.Fatalf("attempt 3: expected 4s, got %v", got)
	}

	if p.exhausted(2) {
		t.Fatal("2 attempts should not be exhausted (max 3)")
	}
	if !p.exhausted(3) {
		t.Fatal("3 attempts should be exhausted (max 3)")
	}
}

func seedFailedDelivery(t *testing.T, subs *memory.WebhookSubscriptionRepository, deliveries *memory.WebhookDeliveryRepository, attempts int) *domain.Subscription {
	t.Helper()
	ctx := context.Background()

	sub, err := domain.NewSubscription("https://merchant.test/hook", "secret", "payment.completed")
	if err != nil {
		t.Fatal(err)
	}
	if err := subs.Save(ctx, sub); err != nil {
		t.Fatal(err)
	}

	d := domain.NewDelivery(sub.ID, "evt-1", "payment.completed", []byte(`{"id":"p"}`))
	for i := 0; i < attempts; i++ {
		d.MarkForRetry("boom", time.Now().Add(-time.Minute)) // vencido → elegível
	}
	if err := deliveries.Save(ctx, d); err != nil {
		t.Fatal(err)
	}
	return sub
}

func TestRetryDeliveriesSucceeds(t *testing.T) {
	ctx := context.Background()
	subs := memory.NewWebhookSubscriptionRepository()
	deliveries := memory.NewWebhookDeliveryRepository(subs)
	seedFailedDelivery(t, subs, deliveries, 1)

	sender := &stubSender{status: 200}
	uc := NewRetryDeliveries(deliveries, sender, BackoffPolicy{MaxAttempts: 5, BaseDelay: time.Minute}, time.Second, 10)

	if err := uc.retryBatch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("expected 1 send, got %d", sender.calls)
	}

	all := deliveries.All()
	if len(all) != 1 || all[0].Status != domain.DeliveryDelivered {
		t.Fatalf("expected delivered after retry, got %+v", all[0])
	}
}

func TestRetryDeliveriesExhausts(t *testing.T) {
	ctx := context.Background()
	subs := memory.NewWebhookSubscriptionRepository()
	deliveries := memory.NewWebhookDeliveryRepository(subs)
	// Já com 2 tentativas; próxima falha (3ª) atinge o máximo → exhausted.
	seedFailedDelivery(t, subs, deliveries, 2)

	sender := &stubSender{status: 500}
	uc := NewRetryDeliveries(deliveries, sender, BackoffPolicy{MaxAttempts: 3, BaseDelay: time.Minute}, time.Second, 10)

	if err := uc.retryBatch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := deliveries.All()
	if len(all) != 1 || all[0].Status != domain.DeliveryExhausted {
		t.Fatalf("expected exhausted after max attempts, got %+v", all[0])
	}

	// Esgotada não é mais elegível a retry.
	items, _ := deliveries.FetchRetriable(ctx, 10, time.Now())
	if len(items) != 0 {
		t.Fatalf("expected no retriable after exhaustion, got %d", len(items))
	}
}

func TestRetryDeliveriesReschedulesOnFailure(t *testing.T) {
	ctx := context.Background()
	subs := memory.NewWebhookSubscriptionRepository()
	deliveries := memory.NewWebhookDeliveryRepository(subs)
	seedFailedDelivery(t, subs, deliveries, 1)

	sender := &stubSender{status: 500}
	uc := NewRetryDeliveries(deliveries, sender, BackoffPolicy{MaxAttempts: 5, BaseDelay: time.Minute}, time.Second, 10)

	if err := uc.retryBatch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := deliveries.All()
	if all[0].Status != domain.DeliveryFailed {
		t.Fatalf("expected still failed (rescheduled), got %s", all[0].Status)
	}
	if all[0].Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", all[0].Attempts)
	}
	if all[0].NextAttemptAt.Before(time.Now()) {
		t.Fatal("expected next attempt scheduled in the future")
	}
}