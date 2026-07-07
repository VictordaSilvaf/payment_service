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

	if err := p.Complete(); err != ErrInvalidTransition {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
	if err := p.Fail(); err != ErrInvalidTransition {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestStatusIsValid(t *testing.T) {
	valid := []Status{
		StatusPending,
		StatusAuthorized,
		StatusCompleted,
		StatusFailed,
		StatusRefunded,
		StatusPartiallyRefunded,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Fatalf("expected %s to be valid", s)
		}
	}

	if Status("invalid").IsValid() {
		t.Fatal("expected invalid status to be false")
	}
}

func TestNewWithOptions(t *testing.T) {
	t.Run("valid manual installments", func(t *testing.T) {
		p, err := NewWithOptions(1000, "BRL", 3, CaptureManual)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Installments != 3 || p.CaptureMethod != CaptureManual {
			t.Fatalf("unexpected payment: %+v", p)
		}
		if p.Status != StatusPending {
			t.Fatalf("expected pending, got %s", p.Status)
		}
	})

	t.Run("invalid installments", func(t *testing.T) {
		if _, err := NewWithOptions(1000, "BRL", 0, CaptureAutomatic); err != ErrInvalidInstallments {
			t.Fatalf("expected ErrInvalidInstallments, got %v", err)
		}
		if _, err := NewWithOptions(1000, "BRL", 13, CaptureAutomatic); err != ErrInvalidInstallments {
			t.Fatalf("expected ErrInvalidInstallments, got %v", err)
		}
	})

	t.Run("invalid capture method", func(t *testing.T) {
		if _, err := NewWithOptions(1000, "BRL", 1, CaptureMethod("later")); err != ErrInvalidCaptureMethod {
			t.Fatalf("expected ErrInvalidCaptureMethod, got %v", err)
		}
	})

	t.Run("defaults from New", func(t *testing.T) {
		p, _ := New(1000, "BRL")
		if p.Installments != 1 || p.CaptureMethod != CaptureAutomatic {
			t.Fatalf("unexpected defaults: %+v", p)
		}
	})
}

func TestPaymentManualCaptureFlow(t *testing.T) {
	p, _ := NewWithOptions(1000, "BRL", 1, CaptureManual)

	if err := p.MarkAuthorized(); err != nil {
		t.Fatalf("authorize failed: %v", err)
	}
	if p.Status != StatusAuthorized {
		t.Fatalf("expected authorized, got %s", p.Status)
	}

	// Não se pode capturar duas vezes nem autorizar de novo.
	if err := p.MarkAuthorized(); err != ErrInvalidTransition {
		t.Fatalf("expected ErrInvalidTransition on re-authorize, got %v", err)
	}

	if err := p.Capture(); err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if p.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", p.Status)
	}

	if err := p.Capture(); err != ErrInvalidTransition {
		t.Fatalf("expected ErrInvalidTransition on re-capture, got %v", err)
	}
}

func TestPaymentCaptureRequiresAuthorized(t *testing.T) {
	p, _ := New(1000, "BRL") // pending
	if err := p.Capture(); err != ErrInvalidTransition {
		t.Fatalf("expected ErrInvalidTransition capturing a pending payment, got %v", err)
	}
}

func TestPaymentRefund(t *testing.T) {
	t.Run("full refund", func(t *testing.T) {
		p, _ := New(1000, "BRL")
		_ = p.Complete()

		if err := p.Refund(1000); err != nil {
			t.Fatalf("refund failed: %v", err)
		}
		if p.Status != StatusRefunded {
			t.Fatalf("expected refunded, got %s", p.Status)
		}
		if p.RefundedAmount != 1000 || p.RefundableAmount() != 0 {
			t.Fatalf("unexpected balances: refunded=%d refundable=%d", p.RefundedAmount, p.RefundableAmount())
		}
	})

	t.Run("partial then full refund", func(t *testing.T) {
		p, _ := New(1000, "BRL")
		_ = p.Complete()

		if err := p.Refund(400); err != nil {
			t.Fatalf("first refund failed: %v", err)
		}
		if p.Status != StatusPartiallyRefunded || p.RefundableAmount() != 600 {
			t.Fatalf("unexpected state: status=%s refundable=%d", p.Status, p.RefundableAmount())
		}

		if err := p.Refund(600); err != nil {
			t.Fatalf("second refund failed: %v", err)
		}
		if p.Status != StatusRefunded {
			t.Fatalf("expected refunded, got %s", p.Status)
		}
	})

	t.Run("refund exceeds balance", func(t *testing.T) {
		p, _ := New(1000, "BRL")
		_ = p.Complete()
		if err := p.Refund(1500); err != ErrRefundExceedsAmount {
			t.Fatalf("expected ErrRefundExceedsAmount, got %v", err)
		}
	})

	t.Run("invalid refund amount", func(t *testing.T) {
		p, _ := New(1000, "BRL")
		_ = p.Complete()
		if err := p.Refund(0); err != ErrInvalidRefundAmount {
			t.Fatalf("expected ErrInvalidRefundAmount, got %v", err)
		}
	})

	t.Run("cannot refund a pending payment", func(t *testing.T) {
		p, _ := New(1000, "BRL")
		if err := p.Refund(100); err != ErrInvalidTransition {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})
}
