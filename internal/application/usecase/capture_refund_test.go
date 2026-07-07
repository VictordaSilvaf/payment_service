package usecase

import (
	"context"
	"errors"
	"testing"

	"payment_service/internal/domain/payment"
	"payment_service/internal/domain/psp"
	"payment_service/internal/infrastructure/persistence/memory"
)

func saveWithStatus(t *testing.T, repo *memory.PaymentRepository, p *payment.Payment) {
	t.Helper()
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}

func TestProcessPaymentManualCaptureAuthorizes(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepository()
	p, _ := payment.NewWithOptions(1000, "BRL", 1, payment.CaptureManual)
	saveWithStatus(t, repo, p)

	outboxRepo := memory.NewOutboxRepository()
	gateway := stubGateway{result: psp.AuthorizationResult{Outcome: psp.OutcomeApproved}}
	uc := NewProcessPayment(repo, gateway, outboxRepo, passthroughTx{})

	out, err := uc.Execute(ctx, ProcessPaymentInput{PaymentID: p.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != string(payment.StatusAuthorized) {
		t.Fatalf("expected authorized, got %s", out.Status)
	}

	events, _ := outboxRepo.FetchUnpublished(ctx, 10)
	if len(events) != 1 || events[0].Type != eventPaymentAuthorized {
		t.Fatalf("expected 1 payment.authorized event, got %+v", events)
	}
}

func TestCapturePaymentExecute(t *testing.T) {
	ctx := context.Background()

	t.Run("captures an authorized payment", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p, _ := payment.NewWithOptions(1000, "BRL", 1, payment.CaptureManual)
		_ = p.MarkAuthorized()
		saveWithStatus(t, repo, p)

		outboxRepo := memory.NewOutboxRepository()
		gateway := stubGateway{captureID: "cap_1"}
		uc := NewCapturePayment(repo, gateway, outboxRepo, passthroughTx{})

		res, err := uc.Execute(ctx, p.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != string(payment.StatusCompleted) {
			t.Fatalf("expected completed, got %s", res.Status)
		}

		events, _ := outboxRepo.FetchUnpublished(ctx, 10)
		if len(events) != 1 || events[0].Type != eventPaymentCompleted {
			t.Fatalf("expected 1 payment.completed event, got %+v", events)
		}
	})

	t.Run("rejects capturing a pending payment", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p, _ := payment.New(1000, "BRL") // pending
		saveWithStatus(t, repo, p)

		uc := NewCapturePayment(repo, stubGateway{}, memory.NewOutboxRepository(), passthroughTx{})
		_, err := uc.Execute(ctx, p.ID)
		if !errors.Is(err, payment.ErrInvalidTransition) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("psp error keeps payment authorized", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p, _ := payment.NewWithOptions(1000, "BRL", 1, payment.CaptureManual)
		_ = p.MarkAuthorized()
		saveWithStatus(t, repo, p)

		pspErr := errors.New("psp down")
		uc := NewCapturePayment(repo, stubGateway{captureErr: pspErr}, memory.NewOutboxRepository(), passthroughTx{})
		if _, err := uc.Execute(ctx, p.ID); !errors.Is(err, pspErr) {
			t.Fatalf("expected psp error, got %v", err)
		}
	})

	t.Run("payment not found", func(t *testing.T) {
		uc := NewCapturePayment(memory.NewPaymentRepository(), stubGateway{}, nil, nil)
		if _, err := uc.Execute(ctx, "missing"); !errors.Is(err, payment.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestRefundPaymentExecute(t *testing.T) {
	ctx := context.Background()

	completed := func(t *testing.T, repo *memory.PaymentRepository) *payment.Payment {
		t.Helper()
		p, _ := payment.New(1000, "BRL")
		_ = p.Complete()
		saveWithStatus(t, repo, p)
		return p
	}

	t.Run("full refund when amount omitted", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p := completed(t, repo)
		outboxRepo := memory.NewOutboxRepository()
		uc := NewRefundPayment(repo, stubGateway{refundID: "ref_1"}, outboxRepo, passthroughTx{})

		res, err := uc.Execute(ctx, p.ID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != string(payment.StatusRefunded) || res.RefundedAmount != 1000 {
			t.Fatalf("unexpected result: %+v", res)
		}

		events, _ := outboxRepo.FetchUnpublished(ctx, 10)
		if len(events) != 1 || events[0].Type != eventPaymentRefunded {
			t.Fatalf("expected 1 payment.refunded event, got %+v", events)
		}
	})

	t.Run("partial refund", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p := completed(t, repo)
		uc := NewRefundPayment(repo, stubGateway{refundID: "ref_1"}, memory.NewOutboxRepository(), passthroughTx{})

		res, err := uc.Execute(ctx, p.ID, 300)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != string(payment.StatusPartiallyRefunded) || res.RefundedAmount != 300 {
			t.Fatalf("unexpected result: %+v", res)
		}
	})

	t.Run("refund exceeding balance is rejected", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p := completed(t, repo)
		uc := NewRefundPayment(repo, stubGateway{}, memory.NewOutboxRepository(), passthroughTx{})
		if _, err := uc.Execute(ctx, p.ID, 2000); !errors.Is(err, payment.ErrRefundExceedsAmount) {
			t.Fatalf("expected ErrRefundExceedsAmount, got %v", err)
		}
	})

	t.Run("cannot refund a pending payment", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p, _ := payment.New(1000, "BRL")
		saveWithStatus(t, repo, p)
		uc := NewRefundPayment(repo, stubGateway{}, memory.NewOutboxRepository(), passthroughTx{})
		if _, err := uc.Execute(ctx, p.ID, 100); !errors.Is(err, payment.ErrInvalidTransition) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("psp error does not persist refund", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p := completed(t, repo)
		pspErr := errors.New("psp down")
		uc := NewRefundPayment(repo, stubGateway{refundErr: pspErr}, memory.NewOutboxRepository(), passthroughTx{})
		if _, err := uc.Execute(ctx, p.ID, 100); !errors.Is(err, pspErr) {
			t.Fatalf("expected psp error, got %v", err)
		}

		stored, _ := repo.FindByID(ctx, p.ID)
		if stored.Status != payment.StatusCompleted {
			t.Fatalf("expected payment to stay completed, got %s", stored.Status)
		}
	})
}
