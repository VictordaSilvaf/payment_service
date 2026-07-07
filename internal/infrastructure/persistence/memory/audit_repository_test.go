package memory

import (
	"context"
	"testing"

	"payment_service/internal/domain/audit"
)

func TestAuditRepositoryAppendIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := NewAuditRepository()

	e := audit.NewAuditEntry("evt-1", audit.AggregatePayment, "pay-1", "payment.completed", []byte(`{"id":"pay-1"}`))
	if err := repo.Append(ctx, e); err != nil {
		t.Fatal(err)
	}
	// Reanexar o mesmo id não duplica nem sobrescreve.
	if err := repo.Append(ctx, e); err != nil {
		t.Fatal(err)
	}

	all := repo.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(all))
	}
}

func TestAuditRepositoryPreservesInsertionOrder(t *testing.T) {
	ctx := context.Background()
	repo := NewAuditRepository()

	_ = repo.Append(ctx, audit.NewAuditEntry("evt-1", audit.AggregatePayment, "pay-1", "payment.created", []byte(`{}`)))
	_ = repo.Append(ctx, audit.NewAuditEntry("evt-2", audit.AggregatePayment, "pay-1", "payment.completed", []byte(`{}`)))

	all := repo.All()
	if len(all) != 2 || all[0].ID != "evt-1" || all[1].ID != "evt-2" {
		t.Fatalf("unexpected order: %+v", all)
	}
}
