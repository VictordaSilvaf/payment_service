package notification

import "context"

// Repository é a porta de persistência do log de notificações.
type Repository interface {
	// Save grava ou atualiza (por id determinístico) o registro da notificação.
	Save(ctx context.Context, n *Notification) error
}
