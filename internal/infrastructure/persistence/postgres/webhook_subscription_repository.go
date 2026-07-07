package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"payment_service/internal/domain/webhook"
)

type WebhookSubscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookSubscriptionRepository(pool *pgxpool.Pool) *WebhookSubscriptionRepository {
	return &WebhookSubscriptionRepository{pool: pool}
}

func (r *WebhookSubscriptionRepository) Save(ctx context.Context, sub *webhook.Subscription) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_subscriptions (id, url, secret, event_type, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, sub.ID, sub.URL, sub.Secret, sub.EventType, sub.Active, sub.CreatedAt)
	if err != nil {
		return fmt.Errorf("save webhook subscription: %w", err)
	}
	return nil
}

func (r *WebhookSubscriptionRepository) FindAll(ctx context.Context) ([]*webhook.Subscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, url, secret, event_type, active, created_at
		FROM webhook_subscriptions
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("find webhook subscriptions: %w", err)
	}
	return scanSubscriptions(rows)
}

func (r *WebhookSubscriptionRepository) FindActiveByEventType(ctx context.Context, eventType string) ([]*webhook.Subscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, url, secret, event_type, active, created_at
		FROM webhook_subscriptions
		WHERE active AND event_type = $1
	`, eventType)
	if err != nil {
		return nil, fmt.Errorf("find active webhook subscriptions: %w", err)
	}
	return scanSubscriptions(rows)
}

func scanSubscriptions(rows pgx.Rows) ([]*webhook.Subscription, error) {
	defer rows.Close()

	subs := make([]*webhook.Subscription, 0)
	for rows.Next() {
		var s webhook.Subscription
		if err := rows.Scan(&s.ID, &s.URL, &s.Secret, &s.EventType, &s.Active, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook subscription: %w", err)
		}
		subs = append(subs, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook subscriptions: %w", err)
	}
	return subs, nil
}
