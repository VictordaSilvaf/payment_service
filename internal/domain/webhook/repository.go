package webhook

import (
	"context"
	"time"
)

// SubscriptionRepository é a porta de persistência das assinaturas.
type SubscriptionRepository interface {
	Save(ctx context.Context, sub *Subscription) error
	FindAll(ctx context.Context) ([]*Subscription, error)
	// FindActiveByEventType retorna apenas assinaturas ativas do tipo informado.
	FindActiveByEventType(ctx context.Context, eventType string) ([]*Subscription, error)
}

// RetriableDelivery reúne uma entrega pendente de retry com os dados atuais do
// destino (URL e segredo da assinatura), evitando que o processo de retry
// precise consultar as assinaturas separadamente.
type RetriableDelivery struct {
	Delivery *Delivery
	URL      string
	Secret   string
}

// DeliveryRepository é a porta de persistência do log de entregas.
type DeliveryRepository interface {
	// Save grava ou atualiza (por subscription_id + event_id) o registro da entrega.
	Save(ctx context.Context, delivery *Delivery) error
	// FetchRetriable retorna entregas com status "failed" cujo NextAttemptAt já
	// passou (elegíveis a nova tentativa), até o limite informado.
	FetchRetriable(ctx context.Context, limit int, now time.Time) ([]RetriableDelivery, error)
}
