package usecase

import (
	"context"

	"payment_service/internal/domain/payment"
)

type ProcessPaymentInput struct {
	PaymentID string
	Amount    int64
	Currency  string
}

type ProcessPaymentOutput struct {
	PaymentID string
	Status    string
}

type ProcessPayment struct {
	repo payment.Repository
}

func NewProcessPayment(repo payment.Repository) *ProcessPayment {
	return &ProcessPayment{repo: repo}
}

func (uc *ProcessPayment) Execute(ctx context.Context, input ProcessPaymentInput) (ProcessPaymentOutput, error) {
	p, err := uc.repo.FindByID(ctx, input.PaymentID)
	if err != nil {
		return ProcessPaymentOutput{}, err
	}

	if err := p.Complete(); err != nil {
		return ProcessPaymentOutput{}, err
	}

	if err := uc.repo.Update(ctx, p); err != nil {
		return ProcessPaymentOutput{}, err
	}

	return ProcessPaymentOutput{
		PaymentID: p.ID,
		Status:    string(p.Status),
	}, nil
}
