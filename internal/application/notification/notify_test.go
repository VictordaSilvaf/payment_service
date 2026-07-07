package notification

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "payment_service/internal/domain/notification"
	"payment_service/internal/infrastructure/persistence/memory"
)

// stubNotifier registra as chamadas e devolve um erro fixo (para testar falhas).
type stubNotifier struct {
	err   error
	calls int
	last  *domain.Notification
}

func (s *stubNotifier) Send(_ context.Context, n *domain.Notification) error {
	s.calls++
	s.last = n
	return s.err
}

const completedPayload = `{"id":"pay-1","amount":1000,"currency":"BRL","status":"completed"}`

func TestNotifyPaymentSendsAndRecords(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewNotificationRepository()
	notifier := &stubNotifier{}

	uc := NewNotifyPayment(notifier, repo, domain.ChannelEmail)
	if err := uc.Execute(ctx, "payment.completed", []byte(completedPayload)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notifier.calls != 1 {
		t.Fatalf("expected 1 send, got %d", notifier.calls)
	}
	if !strings.Contains(notifier.last.Message, "aprovado") {
		t.Fatalf("unexpected message: %q", notifier.last.Message)
	}

	all := repo.All()
	if len(all) != 1 || all[0].Status != domain.StatusSent {
		t.Fatalf("expected 1 sent notification, got %+v", all)
	}
	if all[0].PaymentID != "pay-1" {
		t.Fatalf("unexpected payment id: %s", all[0].PaymentID)
	}
}

func TestNotifyPaymentRecordsFailureAndReturnsError(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewNotificationRepository()
	notifier := &stubNotifier{err: errors.New("smtp down")}

	uc := NewNotifyPayment(notifier, repo, domain.ChannelEmail)
	err := uc.Execute(ctx, "payment.failed", []byte(`{"id":"pay-2","amount":1500,"currency":"BRL","status":"failed"}`))
	if err == nil {
		t.Fatal("expected error to be propagated for retry/DLQ")
	}

	all := repo.All()
	if len(all) != 1 || all[0].Status != domain.StatusFailed {
		t.Fatalf("expected 1 failed notification, got %+v", all)
	}
	if all[0].LastError == "" {
		t.Fatal("expected last error recorded")
	}
}

func TestNotifyPaymentDeterministicID(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewNotificationRepository()
	notifier := &stubNotifier{}

	uc := NewNotifyPayment(notifier, repo, domain.ChannelEmail)
	if err := uc.Execute(ctx, "payment.completed", []byte(completedPayload)); err != nil {
		t.Fatal(err)
	}
	// Reprocessar o mesmo evento não deve criar um segundo registro (upsert por id).
	if err := uc.Execute(ctx, "payment.completed", []byte(completedPayload)); err != nil {
		t.Fatal(err)
	}

	if all := repo.All(); len(all) != 1 {
		t.Fatalf("expected deduplicated notification, got %d", len(all))
	}
}

func TestNotifyPaymentInvalidPayload(t *testing.T) {
	uc := NewNotifyPayment(&stubNotifier{}, memory.NewNotificationRepository(), domain.ChannelEmail)
	if err := uc.Execute(context.Background(), "payment.completed", []byte(`{invalid`)); err == nil {
		t.Fatal("expected error for invalid payload")
	}
}
