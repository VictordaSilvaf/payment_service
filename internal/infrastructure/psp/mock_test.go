package psp

import (
	"context"
	"testing"
	"time"

	"payment_service/internal/domain/payment"
	domainpsp "payment_service/internal/domain/psp"
)

func newPayment(t *testing.T, amount int64) *payment.Payment {
	t.Helper()
	p, err := payment.New(amount, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMockGatewayApprovesEvenAmount(t *testing.T) {
	g := NewMockGateway(0)
	res, err := g.Authorize(context.Background(), newPayment(t, 1000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != domainpsp.OutcomeApproved {
		t.Fatalf("expected approved, got %s", res.Outcome)
	}
	if res.ProviderID == "" {
		t.Fatal("expected provider id")
	}
}

func TestMockGatewayDeclinesOddAmount(t *testing.T) {
	g := NewMockGateway(0)
	res, err := g.Authorize(context.Background(), newPayment(t, 1001))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != domainpsp.OutcomeDeclined {
		t.Fatalf("expected declined, got %s", res.Outcome)
	}
	if res.Reason == "" {
		t.Fatal("expected decline reason")
	}
}

func TestMockGatewayRespectsContextCancellation(t *testing.T) {
	g := NewMockGateway(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancela antes da latência decorrer

	_, err := g.Authorize(ctx, newPayment(t, 1000))
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
