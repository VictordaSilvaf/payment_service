package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"payment_service/internal/domain/audit"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

// Append grava um registro na trilha. ON CONFLICT DO NOTHING mantém a trilha
// imutável e idempotente: reentregas com o mesmo id não criam duplicatas nem
// alteram o registro já existente.
func (r *AuditRepository) Append(ctx context.Context, entry *audit.AuditEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs
			(id, aggregate_type, aggregate_id, event_type, payload, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`,
		entry.ID, string(entry.AggregateType), entry.AggregateID,
		entry.EventType, string(entry.Payload), entry.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("append audit entry: %w", err)
	}
	return nil
}
