package payment

import (
	"testing"
	"time"
)

func TestNewMoney(t *testing.T) {
	t.Run("valid money", func(t *testing.T) {
		m, err := NewMoney(100, "BRL")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.Amount != 100 || m.Currency != "BRL" {
			t.Fatalf("unexpected money: %+v", m)
		}
	})

	t.Run("invalid amount", func(t *testing.T) {
		_, err := NewMoney(0, "BRL")
		if err != ErrInvalidAmount {
			t.Fatalf("expected ErrInvalidAmount, got %v", err)
		}
	})

	t.Run("missing currency", func(t *testing.T) {
		_, err := NewMoney(100, "")
		if err == nil {
			t.Fatal("expected error for empty currency")
		}
	})
}

func TestNewPayment(t *testing.T) {
	p, err := New(5000, "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected generated id")
	}
	if p.Status != StatusPending {
		t.Fatalf("expected pending status, got %s", p.Status)
	}
	if p.Money.Amount != 5000 {
		t.Fatalf("unexpected amount: %d", p.Money.Amount)
	}
}

func TestPaymentCompleteAndFail(t *testing.T) {
	p, err := New(100, "BRL")
	if err != nil {
		t.Fatal(err)
	}

	if err := p.Complete(); err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if p.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", p.Status)
	}

	p2, err := New(100, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	if err := p2.Fail(); err != nil {
		t.Fatalf("fail failed: %v", err)
	}
	if p2.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", p2.Status)
	}
}

func TestPaymentInvalidStatusTransition(t *testing.T) {
	p := &Payment{
		ID:        "id",
		Money:     Money{Amount: 100, Currency: "BRL"},
		Status:    Status("unknown"),
		CreatedAt: time.Now().UTC(),
	}

	if err := p.Complete(); err != ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
	if err := p.Fail(); err != ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestStatusIsValid(t *testing.T) {
	valid := []Status{StatusPending, StatusCompleted, StatusFailed}
	for _, s := range valid {
		if !s.IsValid() {
			t.Fatalf("expected %s to be valid", s)
		}
	}

	if Status("invalid").IsValid() {
		t.Fatal("expected invalid status to be false")
	}
}
