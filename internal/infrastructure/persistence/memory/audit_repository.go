package memory

import (
	"context"
	"sync"

	"payment_service/internal/domain/audit"
)

// AuditRepository é uma implementação em memória para testes. Append-only e
// idempotente por id, como o Postgres (o primeiro registro de um id prevalece).
type AuditRepository struct {
	mu      sync.Mutex
	entries map[string]*audit.AuditEntry
	order   []string // preserva a ordem de inserção para os testes
}

func NewAuditRepository() *AuditRepository {
	return &AuditRepository{entries: make(map[string]*audit.AuditEntry)}
}

func (r *AuditRepository) Append(_ context.Context, entry *audit.AuditEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.ID]; exists {
		return nil // idempotente: não sobrescreve nem duplica
	}
	stored := *entry
	r.entries[entry.ID] = &stored
	r.order = append(r.order, entry.ID)
	return nil
}

// All expõe os registros na ordem de inserção (auxiliar de testes).
func (r *AuditRepository) All() []*audit.AuditEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*audit.AuditEntry, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.entries[id])
	}
	return out
}
