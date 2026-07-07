package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"payment_service/internal/domain/notification"
)

type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

// Save grava a notificação. Em reprocessamentos (mesmo id determinístico), faz
// upsert: atualiza status/erro, mantendo um único registro por (pagamento, evento, canal).
func (r *NotificationRepository) Save(ctx context.Context, n *notification.Notification) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notifications
			(id, payment_id, event_type, channel, recipient, message, status, last_error, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE
		SET status     = EXCLUDED.status,
		    last_error = EXCLUDED.last_error,
		    updated_at = EXCLUDED.updated_at
	`,
		n.ID, n.PaymentID, n.EventType, string(n.Channel), n.Recipient, n.Message,
		string(n.Status), nullString(n.LastError), n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save notification: %w", err)
	}
	return nil
}
