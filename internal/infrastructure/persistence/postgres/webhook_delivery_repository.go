package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"payment_service/internal/domain/webhook"
)

type WebhookDeliveryRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookDeliveryRepository(pool *pgxpool.Pool) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{pool: pool}
}

// Save grava a entrega. Em reentregas (mesmo subscription_id + event_id), faz
// upsert: atualiza status/erro e incrementa attempts, mantendo o registro único.
func (r *WebhookDeliveryRepository) Save(ctx context.Context, d *webhook.Delivery) error {
	var lastError *string
	if d.LastError != "" {
		lastError = &d.LastError
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_deliveries
			(id, subscription_id, event_id, status, attempts, last_error, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (subscription_id, event_id) DO UPDATE
		SET status     = EXCLUDED.status,
		    attempts   = webhook_deliveries.attempts + 1,
		    last_error = EXCLUDED.last_error,
		    updated_at = EXCLUDED.updated_at
	`, d.ID, d.SubscriptionID, d.EventID, string(d.Status), d.Attempts, lastError, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save webhook delivery: %w", err)
	}
	return nil
}
