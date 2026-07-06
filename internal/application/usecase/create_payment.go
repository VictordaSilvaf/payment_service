package usecase

import (
	"context"

	"payment_service/internal/application/dto"
	"payment_service/internal/domain/payment"
)

type CreatePayment struct {
	repo      payment.Repository
	publisher payment.EventPublisher
}

func NewCreatePayment(repo payment.Repository, publisher payment.EventPublisher) *CreatePayment {
	return &CreatePayment{repo: repo, publisher: publisher}
}

func (uc *CreatePayment) Execute(ctx context.Context, req dto.CreatePaymentRequest) (*dto.PaymentResponse, error) {
	p, err := payment.New(req.Amount, req.Currency)
	if err != nil {
		return nil, err
	}

	if err := uc.repo.Save(ctx, p); err != nil {
		return nil, err
	}

	if uc.publisher != nil {
		if err := uc.publisher.PublishCreated(ctx, p); err != nil {
			return nil, err
		}
	}

	return toPaymentResponse(p), nil
}
