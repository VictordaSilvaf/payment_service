package audit

import (
	"context"
	"testing"

	domain "payment_service/internal/domain/audit"
	"payment_service/internal/infrastructure/persistence/memory"
)

const completedPayload = `{"id":"pay-1","amount":1000,"currency":"BRL","status":"completed"}`

func TestRecordAuditAppends(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAuditRepository()
	uc := NewRecordAudit(repo)

	if err := uc.Execute(ctx, "evt-1", "payment.completed", []byte(completedPayload)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := repo.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(all))
	}
	if all[0].ID != "evt-1" || all[0].AggregateID != "pay-1" {
		t.Fatalf("unexpected entry: %+v", all[0])
	}
	if all[0].EventType != "payment.completed" || all[0].AggregateType != domain.AggregatePayment {
		t.Fatalf("unexpected entry: %+v", all[0])
	}
}

func TestRecordAuditDedupByEventID(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAuditRepository()
	uc := NewRecordAudit(repo)

	// Mesma reentrega (mesmo id) não deve duplicar a trilha.
	for i := 0; i < 3; i++ {
		if err := uc.Execute(ctx, "evt-1", "payment.completed", []byte(completedPayload)); err != nil {
			t.Fatal(err)
		}
	}
	if all := repo.All(); len(all) != 1 {
		t.Fatalf("expected deduplicated entry, got %d", len(all))
	}
}

func TestRecordAuditDistinctEventsAreKept(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAuditRepository()
	uc := NewRecordAudit(repo)

	_ = uc.Execute(ctx, "evt-1", "payment.created", []byte(`{"id":"pay-1"}`))
	_ = uc.Execute(ctx, "evt-2", "payment.completed", []byte(completedPayload))

	if all := repo.All(); len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
}

func TestRecordAuditFallbackIDWhenMessageIDMissing(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAuditRepository()
	uc := NewRecordAudit(repo)

	// Sem id: dedup passa a valer pelo conteúdo (tipo + payload).
	if err := uc.Execute(ctx, "", "payment.completed", []byte(completedPayload)); err != nil {
		t.Fatal(err)
	}
	if err := uc.Execute(ctx, "", "payment.completed", []byte(completedPayload)); err != nil {
		t.Fatal(err)
	}
	if all := repo.All(); len(all) != 1 {
		t.Fatalf("expected content-based dedup, got %d", len(all))
	}

	// Conteúdo diferente → novo registro.
	if err := uc.Execute(ctx, "", "payment.failed", []byte(completedPayload)); err != nil {
		t.Fatal(err)
	}
	if all := repo.All(); len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
}

func TestRecordAuditInvalidPayload(t *testing.T) {
	uc := NewRecordAudit(memory.NewAuditRepository())
	if err := uc.Execute(context.Background(), "evt-1", "payment.completed", []byte(`{invalid`)); err == nil {
		t.Fatal("expected error for invalid payload")
	}
}
