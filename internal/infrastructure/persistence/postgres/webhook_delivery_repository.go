package postgres

import (
	"context"
	"fmt"
	"time"

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
// upsert, persistindo o estado calculado pelo domínio (status, attempts,
// agendamento do próximo retry).
func (r *WebhookDeliveryRepository) Save(ctx context.Context, d *webhook.Delivery) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_deliveries
			(id, subscription_id, event_id, event_type, payload, status, attempts, last_error, next_attempt_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (subscription_id, event_id) DO UPDATE
		SET status          = EXCLUDED.status,
		    attempts        = EXCLUDED.attempts,
		    last_error      = EXCLUDED.last_error,
		    next_attempt_at = EXCLUDED.next_attempt_at,
		    updated_at      = EXCLUDED.updated_at
	`,
		d.ID, d.SubscriptionID, d.EventID, d.EventType, string(d.Payload),
		string(d.Status), d.Attempts, nullString(d.LastError), nullTime(d.NextAttemptAt),
		d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save webhook delivery: %w", err)
	}
	return nil
}

// FetchRetriable retorna entregas com status "failed" já no prazo de retry,
// juntando a assinatura (ativa) para obter URL e segredo atuais.
func (r *WebhookDeliveryRepository) FetchRetriable(ctx context.Context, limit int, now time.Time) ([]webhook.RetriableDelivery, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.subscription_id, d.event_id, d.event_type, d.payload,
		       d.status, d.attempts, d.last_error, d.next_attempt_at, d.created_at, d.updated_at,
		       s.url, s.secret
		FROM webhook_deliveries d
		JOIN webhook_subscriptions s ON s.id = d.subscription_id
		WHERE d.status = 'failed'
		  AND s.active
		  AND d.next_attempt_at IS NOT NULL
		  AND d.next_attempt_at <= $1
		ORDER BY d.next_attempt_at
		LIMIT $2
	`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch retriable deliveries: %w", err)
	}
	defer rows.Close()

	items := make([]webhook.RetriableDelivery, 0, limit)
	for rows.Next() {
		var (
			d             webhook.Delivery
			payload       string
			lastError     *string
			nextAttemptAt *time.Time
			url, secret   string
		)
		if err := rows.Scan(
			&d.ID, &d.SubscriptionID, &d.EventID, &d.EventType, &payload,
			&d.Status, &d.Attempts, &lastError, &nextAttemptAt, &d.CreatedAt, &d.UpdatedAt,
			&url, &secret,
		); err != nil {
			return nil, fmt.Errorf("scan retriable delivery: %w", err)
		}

		d.Payload = []byte(payload)
		if lastError != nil {
			d.LastError = *lastError
		}
		if nextAttemptAt != nil {
			d.NextAttemptAt = *nextAttemptAt
		}

		items = append(items, webhook.RetriableDelivery{Delivery: &d, URL: url, Secret: secret})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retriable deliveries: %w", err)
	}
	return items, nil
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
