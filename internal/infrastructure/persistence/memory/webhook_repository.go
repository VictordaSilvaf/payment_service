package memory

import (
	"context"
	"sync"
	"time"

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

// findActiveByID retorna a assinatura ativa com o id informado (ou nil). Auxiliar
// interno usado pelo WebhookDeliveryRepository para o "join" do FetchRetriable.
func (r *WebhookSubscriptionRepository) findActiveByID(id string) *webhook.Subscription {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, s := range r.subs {
		if s.ID == id && s.Active {
			return s
		}
	}
	return nil
}

// WebhookDeliveryRepository é uma implementação em memória para testes. Guarda as
// entregas indexadas por (subscription_id, event_id) para reproduzir o upsert do
// Postgres, e usa o repositório de assinaturas para o "join" do FetchRetriable.
type WebhookDeliveryRepository struct {
	mu         sync.Mutex
	deliveries map[string]*webhook.Delivery
	subs       *WebhookSubscriptionRepository
}

func NewWebhookDeliveryRepository(subs *WebhookSubscriptionRepository) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{
		deliveries: make(map[string]*webhook.Delivery),
		subs:       subs,
	}
}

func (r *WebhookDeliveryRepository) Save(_ context.Context, d *webhook.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Upsert por (subscription_id, event_id), como no Postgres.
	stored := *d
	r.deliveries[d.SubscriptionID+":"+d.EventID] = &stored
	return nil
}

func (r *WebhookDeliveryRepository) FetchRetriable(_ context.Context, limit int, now time.Time) ([]webhook.RetriableDelivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]webhook.RetriableDelivery, 0, limit)
	for _, d := range r.deliveries {
		if d.Status != webhook.DeliveryFailed || d.NextAttemptAt.IsZero() || d.NextAttemptAt.After(now) {
			continue
		}
		sub := r.subs.findActiveByID(d.SubscriptionID)
		if sub == nil {
			continue
		}
		clone := *d
		items = append(items, webhook.RetriableDelivery{Delivery: &clone, URL: sub.URL, Secret: sub.Secret})
		if len(items) == limit {
			break
		}
	}
	return items, nil
}

// All expõe as entregas gravadas (auxiliar de testes).
func (r *WebhookDeliveryRepository) All() []*webhook.Delivery {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*webhook.Delivery, 0, len(r.deliveries))
	for _, d := range r.deliveries {
		out = append(out, d)
	}
	return out
}
