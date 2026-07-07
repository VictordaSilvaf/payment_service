package webhook

import (
	"context"
	"errors"
	"testing"

	domain "payment_service/internal/domain/webhook"
	"payment_service/internal/infrastructure/persistence/memory"
)

type stubSender struct {
	status int
	err    error
	calls  int
	last   domain.SendRequest
}

func (s *stubSender) Send(_ context.Context, req domain.SendRequest) (int, error) {
	s.calls++
	s.last = req
	return s.status, s.err
}

func newSubscription(t *testing.T, repo *memory.WebhookSubscriptionRepository, eventType string) *domain.Subscription {
	t.Helper()
	sub, err := domain.NewSubscription("https://merchant.test/hook", "secret", eventType)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	return sub
}

func TestDispatchDeliversToActiveSubscriptions(t *testing.T) {
	ctx := context.Background()
	subs := memory.NewWebhookSubscriptionRepository()
	newSubscription(t, subs, "payment.completed")
	newSubscription(t, subs, "payment.created") // não deve receber

	deliveries := memory.NewWebhookDeliveryRepository()
	sender := &stubSender{status: 200}

	uc := NewDispatchWebhook(subs, deliveries, sender)
	payload := []byte(`{"id":"pay-1","status":"completed"}`)

	if err := uc.Execute(ctx, "payment.completed", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.calls != 1 {
		t.Fatalf("expected 1 send, got %d", sender.calls)
	}
	all := deliveries.All()
	if len(all) != 1 || all[0].Status != domain.DeliveryDelivered {
		t.Fatalf("expected 1 delivered, got %+v", all)
	}
	if sender.last.Signature == "" || sender.last.WebhookID == "" {
		t.Fatalf("expected signature and webhook id, got %+v", sender.last)
	}
}

func TestDispatchRecordsFailureOnNon2xx(t *testing.T) {
	ctx := context.Background()
	subs := memory.NewWebhookSubscriptionRepository()
	newSubscription(t, subs, "payment.completed")
	deliveries := memory.NewWebhookDeliveryRepository()

	uc := NewDispatchWebhook(subs, deliveries, &stubSender{status: 500})
	if err := uc.Execute(ctx, "payment.completed", []byte(`{"id":"p"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := deliveries.All()
	if len(all) != 1 || all[0].Status != domain.DeliveryFailed {
		t.Fatalf("expected 1 failed delivery, got %+v", all)
	}
}

func TestDispatchRecordsFailureOnSenderError(t *testing.T) {
	ctx := context.Background()
	subs := memory.NewWebhookSubscriptionRepository()
	newSubscription(t, subs, "payment.completed")
	deliveries := memory.NewWebhookDeliveryRepository()

	uc := NewDispatchWebhook(subs, deliveries, &stubSender{err: errors.New("timeout")})
	if err := uc.Execute(ctx, "payment.completed", []byte(`{"id":"p"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := deliveries.All()
	if len(all) != 1 || all[0].Status != domain.DeliveryFailed {
		t.Fatalf("expected failed delivery, got %+v", all)
	}
	if all[0].LastError != "timeout" {
		t.Fatalf("expected last error 'timeout', got %q", all[0].LastError)
	}
}

func TestDispatchStableDeliveryID(t *testing.T) {
	ctx := context.Background()
	subs := memory.NewWebhookSubscriptionRepository()
	sub := newSubscription(t, subs, "payment.completed")
	sender := &stubSender{status: 200}
	uc := NewDispatchWebhook(subs, memory.NewWebhookDeliveryRepository(), sender)
	payload := []byte(`{"id":"pay-1"}`)

	if err := uc.Execute(ctx, "payment.completed", payload); err != nil {
		t.Fatal(err)
	}
	first := sender.last.WebhookID

	if err := uc.Execute(ctx, "payment.completed", payload); err != nil {
		t.Fatal(err)
	}
	if sender.last.WebhookID != first {
		t.Fatal("expected stable webhook id across redeliveries")
	}

	// Depende da assinatura e do id do pagamento.
	want := deliveryID(sub.ID, "payment.completed", "pay-1")
	if first != want {
		t.Fatalf("unexpected webhook id: got %s want %s", first, want)
	}
}

func TestDispatchReturnsErrorWhenSubsRepoFails(t *testing.T) {
	uc := NewDispatchWebhook(&errorSubsRepo{}, memory.NewWebhookDeliveryRepository(), &stubSender{status: 200})
	if err := uc.Execute(context.Background(), "payment.completed", []byte(`{}`)); err == nil {
		t.Fatal("expected error when subscription repo fails")
	}
}

type errorSubsRepo struct{}

func (errorSubsRepo) Save(context.Context, *domain.Subscription) error { return nil }
func (errorSubsRepo) FindAll(context.Context) ([]*domain.Subscription, error) {
	return nil, errors.New("db down")
}
func (errorSubsRepo) FindActiveByEventType(context.Context, string) ([]*domain.Subscription, error) {
	return nil, errors.New("db down")
}
