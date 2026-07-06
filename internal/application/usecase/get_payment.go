package usecase

import (
	"context"
	"errors"

	"payment_service/internal/application/dto"
	"payment_service/internal/domain/payment"
)

type GetPayment struct {
	repo payment.Repository
}

func NewGetPayment(repo payment.Repository) *GetPayment {
	return &GetPayment{repo: repo}
}

func (uc *GetPayment) Execute(ctx context.Context, id string) (*dto.PaymentResponse, error) {
	p, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, payment.ErrNotFound) {
			return nil, payment.ErrNotFound
		}
		return nil, err
	}

	return toPaymentResponse(p), nil
}
