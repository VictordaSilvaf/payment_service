package memory

import (
	"context"
	"testing"

	"payment_service/internal/domain/payment"
)

func TestPaymentRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewPaymentRepository()

	p, err := payment.New(1000, "BRL")
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	found, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if found.ID != p.ID {
		t.Fatalf("expected id %s, got %s", p.ID, found.ID)
	}

	_, err = repo.FindByID(ctx, "missing")
	if err != payment.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	for i := range 3 {
		other, makeErr := payment.New(int64(100*(i+1)), "USD")
		if makeErr != nil {
			t.Fatal(makeErr)
		}
		if saveErr := repo.Save(ctx, other); saveErr != nil {
			t.Fatal(saveErr)
		}
	}

	page, err := repo.FindAll(ctx, "1", "2", "", "", "")
	if err != nil {
		t.Fatalf("find all failed: %v", err)
	}
	if page.Total != 4 || len(page.Items) != 2 {
		t.Fatalf("unexpected page: total=%d len=%d", page.Total, len(page.Items))
	}

	page, err = repo.FindAll(ctx, "10", "10", "", "", "")
	if err != nil {
		t.Fatalf("find all failed: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("expected empty page, got %d items", len(page.Items))
	}

	if err := repo.Delete(ctx, p.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestParsePositiveIntMemory(t *testing.T) {
	if parsePositiveInt("2", 1) != 2 {
		t.Fatal("expected 2")
	}
	if parsePositiveInt("x", 5) != 5 {
		t.Fatal("expected fallback")
	}
}
