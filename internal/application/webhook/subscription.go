package webhook

import (
	"context"

	"payment_service/internal/application/dto"
	domain "payment_service/internal/domain/webhook"
)

// CreateSubscription registra uma nova assinatura de webhook.
type CreateSubscription struct {
	repo domain.SubscriptionRepository
}

func NewCreateSubscription(repo domain.SubscriptionRepository) *CreateSubscription {
	return &CreateSubscription{repo: repo}
}

func (uc *CreateSubscription) Execute(ctx context.Context, req dto.CreateWebhookRequest) (*dto.WebhookResponse, error) {
	sub, err := domain.NewSubscription(req.URL, req.Secret, req.EventType)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.Save(ctx, sub); err != nil {
		return nil, err
	}
	return toWebhookResponse(sub), nil
}

// ListSubscriptions lista todas as assinaturas cadastradas.
type ListSubscriptions struct {
	repo domain.SubscriptionRepository
}

func NewListSubscriptions(repo domain.SubscriptionRepository) *ListSubscriptions {
	return &ListSubscriptions{repo: repo}
}

func (uc *ListSubscriptions) Execute(ctx context.Context) ([]*dto.WebhookResponse, error) {
	subs, err := uc.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.WebhookResponse, 0, len(subs))
	for _, s := range subs {
		out = append(out, toWebhookResponse(s))
	}
	return out, nil
}

func toWebhookResponse(s *domain.Subscription) *dto.WebhookResponse {
	return &dto.WebhookResponse{
		ID:        s.ID,
		URL:       s.URL,
		EventType: s.EventType,
		Secret:    s.Secret,
		Active:    s.Active,
		CreatedAt: s.CreatedAt,
	}
}
