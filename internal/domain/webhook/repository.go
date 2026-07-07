package webhook

import "context"

// SubscriptionRepository é a porta de persistência das assinaturas.
type SubscriptionRepository interface {
	Save(ctx context.Context, sub *Subscription) error
	FindAll(ctx context.Context) ([]*Subscription, error)
	// FindActiveByEventType retorna apenas assinaturas ativas do tipo informado.
	FindActiveByEventType(ctx context.Context, eventType string) ([]*Subscription, error)
}

// DeliveryRepository é a porta de persistência do log de entregas.
type DeliveryRepository interface {
	// Save grava (ou atualiza, em reentregas) o registro da entrega.
	Save(ctx context.Context, delivery *Delivery) error
}
