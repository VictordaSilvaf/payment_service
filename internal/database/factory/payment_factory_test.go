package factory

import (
	"testing"
	"time"

	"payment_service/internal/domain/payment"
)

func TestPaymentFactory(t *testing.T) {
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	p := NewPaymentFactory().
		WithAmount(5000).
		WithCurrency("BRL").
		WithStatus(payment.StatusCompleted).
		WithCreatedAt(created).
		Make()

	if p.Money.Amount != 5000 || p.Money.Currency != "BRL" {
		t.Fatalf("unexpected money: %+v", p.Money)
	}
	if p.Status != payment.StatusCompleted {
		t.Fatalf("unexpected status: %s", p.Status)
	}
	if !p.CreatedAt.Equal(created) {
		t.Fatalf("unexpected created_at: %s", p.CreatedAt)
	}
	if p.ID == "" {
		t.Fatal("expected generated id")
	}
}

func TestPaymentFactoryMakeMany(t *testing.T) {
	payments := NewPaymentFactory().WithAmount(100).WithCurrency("USD").MakeMany(3)
	if len(payments) != 3 {
		t.Fatalf("expected 3 payments, got %d", len(payments))
	}
	for _, p := range payments {
		if p.Money.Amount != 100 || p.Money.Currency != "USD" {
			t.Fatalf("unexpected payment: %+v", p)
		}
	}
}

func TestPaymentFactoryDefaults(t *testing.T) {
	p := NewPaymentFactory().Make()
	if p.Money.Amount <= 0 {
		t.Fatalf("expected positive amount, got %d", p.Money.Amount)
	}
	if p.Money.Currency == "" {
		t.Fatal("expected currency")
	}
	if !p.Status.IsValid() {
		t.Fatalf("invalid status: %s", p.Status)
	}
}
