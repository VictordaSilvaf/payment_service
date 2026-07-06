package usecase

import (
	"context"

	"payment_service/internal/application/dto"
	"payment_service/internal/domain/payment"
)

type CreatePayment struct {
	repo payment.Repository
}

func NewCreatePayment(repo payment.Repository) *CreatePayment {
	return &CreatePayment{repo: repo}
}

func (uc *CreatePayment) Execute(ctx context.Context, req dto.CreatePaymentRequest) (*dto.PaymentResponse, error) {
	p, err := payment.New(req.Amount, req.Currency)
	if err != nil {
		return nil, err
	}

	if err := uc.repo.Save(ctx, p); err != nil {
		return nil, err
	}

	return toPaymentResponse(p), nil
}

func toPaymentResponse(p *payment.Payment) *dto.PaymentResponse {
	return &dto.PaymentResponse{
		ID:        p.ID,
		Amount:    p.Money.Amount,
		Currency:  p.Money.Currency,
		Status:    string(p.Status),
		CreatedAt: p.CreatedAt,
	}
}
