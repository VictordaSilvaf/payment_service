package memory

import (
	"context"
	"sync"

	"payment_service/internal/domain/notification"
)

// NotificationRepository é uma implementação em memória para testes. Faz upsert
// por id determinístico, como o Postgres.
type NotificationRepository struct {
	mu            sync.Mutex
	notifications map[string]*notification.Notification
}

func NewNotificationRepository() *NotificationRepository {
	return &NotificationRepository{notifications: make(map[string]*notification.Notification)}
}

func (r *NotificationRepository) Save(_ context.Context, n *notification.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored := *n
	r.notifications[n.ID] = &stored
	return nil
}

// All expõe as notificações gravadas (auxiliar de testes).
func (r *NotificationRepository) All() []*notification.Notification {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*notification.Notification, 0, len(r.notifications))
	for _, n := range r.notifications {
		out = append(out, n)
	}
	return out
}
