package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"payment_service/internal/domain/outbox"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) db(ctx context.Context) executor {
	return execFromContext(ctx, r.pool)
}

// Add usa execFromContext: quando chamado dentro de um TxManager.WithinTx, grava
// na mesma transação do pagamento (atomicidade); fora dela, roda em autocommit.
func (r *OutboxRepository) Add(ctx context.Context, event outbox.Event) error {
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO outbox_events (id, aggregate_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, event.ID, event.AggregateID, event.Type, string(event.Payload), event.CreatedAt)
	if err != nil {
		return fmt.Errorf("add outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished lê os eventos pendentes mais antigos. O relay publica em ordem
// e marca cada um como publicado em seguida.
func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Event, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, aggregate_id, event_type, payload, created_at
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch unpublished: %w", err)
	}
	defer rows.Close()

	events := make([]outbox.Event, 0, limit)
	for rows.Next() {
		var e outbox.Event
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.Type, &e.Payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return events, nil
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark published: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("outbox event %s not found", id)
	}
	return nil
}
