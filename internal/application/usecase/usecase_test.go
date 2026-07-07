package usecase

import (
	"context"
	"errors"
	"testing"

	"payment_service/internal/application/dto"
	"payment_service/internal/domain/outbox"
	"payment_service/internal/domain/payment"
	"payment_service/internal/domain/psp"
	"payment_service/internal/infrastructure/persistence/memory"
	"payment_service/internal/testutil"
)

// stubGateway simula o PSP nos testes, devolvendo um resultado/erro fixo.
type stubGateway struct {
	result     psp.AuthorizationResult
	err        error
	captureID  string
	captureErr error
	refundID   string
	refundErr  error
}

func (g stubGateway) Authorize(_ context.Context, _ *payment.Payment) (psp.AuthorizationResult, error) {
	return g.result, g.err
}

func (g stubGateway) Capture(_ context.Context, _ *payment.Payment) (string, error) {
	return g.captureID, g.captureErr
}

func (g stubGateway) Refund(_ context.Context, _ *payment.Payment, _ int64) (string, error) {
	return g.refundID, g.refundErr
}

// passthroughTx executa a função direto, sem transação real (para testes).
type passthroughTx struct{}

func (passthroughTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// errorOutboxRepo falha ao gravar o evento, para testar o rollback lógico.
type errorOutboxRepo struct {
	addErr error
}

func (r *errorOutboxRepo) Add(_ context.Context, _ outbox.Event) error { return r.addErr }
func (r *errorOutboxRepo) FetchUnpublished(_ context.Context, _ int) ([]outbox.Event, error) {
	return nil, nil
}
func (r *errorOutboxRepo) MarkPublished(_ context.Context, _ string) error { return nil }

func TestCreatePaymentExecute(t *testing.T) {
	ctx := context.Background()

	t.Run("success writes payment and outbox event", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		outboxRepo := memory.NewOutboxRepository()
		uc := NewCreatePayment(repo, outboxRepo, passthroughTx{})

		result, err := uc.Execute(ctx, dto.CreatePaymentRequest{Amount: 1000, Currency: "BRL"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ID == "" || result.Amount != 1000 {
			t.Fatalf("unexpected result: %+v", result)
		}

		events, err := outboxRepo.FetchUnpublished(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 outbox event, got %d", len(events))
		}
		if events[0].Type != eventPaymentCreated || events[0].AggregateID != result.ID {
			t.Fatalf("unexpected outbox event: %+v", events[0])
		}
	})

	t.Run("success without tx manager", func(t *testing.T) {
		uc := NewCreatePayment(memory.NewPaymentRepository(), memory.NewOutboxRepository(), nil)
		_, err := uc.Execute(ctx, dto.CreatePaymentRequest{Amount: 2000, Currency: "USD"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid amount", func(t *testing.T) {
		uc := NewCreatePayment(memory.NewPaymentRepository(), memory.NewOutboxRepository(), nil)
		_, err := uc.Execute(ctx, dto.CreatePaymentRequest{Amount: 0, Currency: "BRL"})
		if !errors.Is(err, payment.ErrInvalidAmount) {
			t.Fatalf("expected ErrInvalidAmount, got %v", err)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("db down")
		uc := NewCreatePayment(&testutil.ErrorPaymentRepository{SaveErr: repoErr}, memory.NewOutboxRepository(), passthroughTx{})
		_, err := uc.Execute(ctx, dto.CreatePaymentRequest{Amount: 100, Currency: "BRL"})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected repo error, got %v", err)
		}
	})

	t.Run("outbox error", func(t *testing.T) {
		outboxErr := errors.New("outbox down")
		uc := NewCreatePayment(memory.NewPaymentRepository(), &errorOutboxRepo{addErr: outboxErr}, passthroughTx{})
		_, err := uc.Execute(ctx, dto.CreatePaymentRequest{Amount: 100, Currency: "BRL"})
		if !errors.Is(err, outboxErr) {
			t.Fatalf("expected outbox error, got %v", err)
		}
	})
}

func TestGetPaymentExecute(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepository()
	uc := NewGetPayment(repo)

	p, err := payment.New(100, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	t.Run("found", func(t *testing.T) {
		result, err := uc.Execute(ctx, p.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ID != p.ID {
			t.Fatalf("expected id %s, got %s", p.ID, result.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := uc.Execute(ctx, "missing-id")
		if !errors.Is(err, payment.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestListPaymentExecute(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPaymentRepository()
	uc := NewListPayment(repo)

	for i := range 5 {
		p, err := payment.New(int64(100*(i+1)), "BRL")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("default pagination", func(t *testing.T) {
		result, err := uc.Execute(ctx, "", "", "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Total != 5 || len(result.Data) != 5 {
			t.Fatalf("unexpected result: total=%d len=%d", result.Total, len(result.Data))
		}
		if result.Page != "1" || result.Limit != "10" {
			t.Fatalf("unexpected defaults: page=%s limit=%s", result.Page, result.Limit)
		}
	})

	t.Run("paginated", func(t *testing.T) {
		result, err := uc.Execute(ctx, "1", "2", "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Data) != 2 || result.TotalPages != 3 {
			t.Fatalf("unexpected pagination: len=%d totalPages=%d", len(result.Data), result.TotalPages)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("list failed")
		errorRepo := &testutil.ErrorPaymentRepository{SaveErr: repoErr}
		errorUC := NewListPayment(errorRepo)
		_, err := errorUC.Execute(ctx, "1", "10", "", "", "")
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected repo error, got %v", err)
		}
	})
}

func TestProcessPaymentExecute(t *testing.T) {
	ctx := context.Background()

	t.Run("success completes payment", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p, err := payment.New(1000, "BRL")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}

		outboxRepo := memory.NewOutboxRepository()
		gateway := stubGateway{result: psp.AuthorizationResult{Outcome: psp.OutcomeApproved}}
		uc := NewProcessPayment(repo, gateway, outboxRepo, passthroughTx{})
		out, err := uc.Execute(ctx, ProcessPaymentInput{PaymentID: p.ID, Amount: 1000, Currency: "BRL"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Status != string(payment.StatusCompleted) {
			t.Fatalf("expected completed, got %s", out.Status)
		}

		stored, err := repo.FindByID(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Status != payment.StatusCompleted {
			t.Fatalf("expected stored payment completed, got %s", stored.Status)
		}

		events, err := outboxRepo.FetchUnpublished(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].Type != eventPaymentCompleted {
			t.Fatalf("expected 1 payment.completed event, got %+v", events)
		}
	})

	t.Run("psp declines and emits payment.failed", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p, err := payment.New(1001, "BRL")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}

		outboxRepo := memory.NewOutboxRepository()
		gateway := stubGateway{result: psp.AuthorizationResult{Outcome: psp.OutcomeDeclined, Reason: "insufficient funds"}}
		uc := NewProcessPayment(repo, gateway, outboxRepo, passthroughTx{})

		out, err := uc.Execute(ctx, ProcessPaymentInput{PaymentID: p.ID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Status != string(payment.StatusFailed) {
			t.Fatalf("expected failed, got %s", out.Status)
		}

		stored, _ := repo.FindByID(ctx, p.ID)
		if stored.Status != payment.StatusFailed {
			t.Fatalf("expected stored payment failed, got %s", stored.Status)
		}

		events, _ := outboxRepo.FetchUnpublished(ctx, 10)
		if len(events) != 1 || events[0].Type != eventPaymentFailed {
			t.Fatalf("expected 1 payment.failed event, got %+v", events)
		}
	})

	t.Run("psp error keeps payment pending", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		p, err := payment.New(1000, "BRL")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}

		pspErr := errors.New("psp unavailable")
		uc := NewProcessPayment(repo, stubGateway{err: pspErr}, memory.NewOutboxRepository(), passthroughTx{})

		_, err = uc.Execute(ctx, ProcessPaymentInput{PaymentID: p.ID})
		if !errors.Is(err, pspErr) {
			t.Fatalf("expected psp error, got %v", err)
		}

		stored, _ := repo.FindByID(ctx, p.ID)
		if stored.Status != payment.StatusPending {
			t.Fatalf("expected payment to remain pending, got %s", stored.Status)
		}
	})

	t.Run("payment not found", func(t *testing.T) {
		repo := memory.NewPaymentRepository()
		uc := NewProcessPayment(repo, nil, nil, nil)

		_, err := uc.Execute(ctx, ProcessPaymentInput{PaymentID: "missing-id"})
		if !errors.Is(err, payment.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("update error", func(t *testing.T) {
		base := memory.NewPaymentRepository()
		p, err := payment.New(1000, "BRL")
		if err != nil {
			t.Fatal(err)
		}
		if err := base.Save(ctx, p); err != nil {
			t.Fatal(err)
		}

		updateErr := errors.New("update failed")
		repo := &updateErrorRepo{PaymentRepository: base, updateErr: updateErr}

		uc := NewProcessPayment(repo, nil, nil, nil)
		_, err = uc.Execute(ctx, ProcessPaymentInput{PaymentID: p.ID})
		if !errors.Is(err, updateErr) {
			t.Fatalf("expected update error, got %v", err)
		}
	})
}

type updateErrorRepo struct {
	*memory.PaymentRepository
	updateErr error
}

func (r *updateErrorRepo) Update(_ context.Context, _ *payment.Payment) error {
	return r.updateErr
}

func TestParsePositiveInt(t *testing.T) {
	if parsePositiveInt("3", 10) != 3 {
		t.Fatal("expected 3")
	}
	if parsePositiveInt("invalid", 10) != 10 {
		t.Fatal("expected fallback")
	}
	if parsePositiveInt("-1", 10) != 10 {
		t.Fatal("expected fallback for negative")
	}
}
