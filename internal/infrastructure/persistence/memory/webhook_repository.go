package memory

import (
	"context"
	"sync"

	"payment_service/internal/domain/webhook"
)

// WebhookSubscriptionRepository é uma implementação em memória para testes.
type WebhookSubscriptionRepository struct {
	mu   sync.Mutex
	subs []*webhook.Subscription
}

func NewWebhookSubscriptionRepository() *WebhookSubscriptionRepository {
	return &WebhookSubscriptionRepository{subs: make([]*webhook.Subscription, 0)}
}

func (r *WebhookSubscriptionRepository) Save(_ context.Context, sub *webhook.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs = append(r.subs, sub)
	return nil
}

func (r *WebhookSubscriptionRepository) FindAll(_ context.Context) ([]*webhook.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*webhook.Subscription, len(r.subs))
	copy(out, r.subs)
	return out, nil
}

func (r *WebhookSubscriptionRepository) FindActiveByEventType(_ context.Context, eventType string) ([]*webhook.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*webhook.Subscription, 0)
	for _, s := range r.subs {
		if s.Active && s.EventType == eventType {
			out = append(out, s)
		}
	}
	return out, nil
}

// WebhookDeliveryRepository é uma implementação em memória para testes.
type WebhookDeliveryRepository struct {
	mu         sync.Mutex
	deliveries []*webhook.Delivery
}

func NewWebhookDeliveryRepository() *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{deliveries: make([]*webhook.Delivery, 0)}
}

func (r *WebhookDeliveryRepository) Save(_ context.Context, d *webhook.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deliveries = append(r.deliveries, d)
	return nil
}

// All expõe as entregas gravadas (auxiliar de testes).
func (r *WebhookDeliveryRepository) All() []*webhook.Delivery {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*webhook.Delivery, len(r.deliveries))
	copy(out, r.deliveries)
	return out
}
