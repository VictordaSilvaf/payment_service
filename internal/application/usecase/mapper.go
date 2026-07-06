package usecase

import (
	"payment_service/internal/application/dto"
	"payment_service/internal/domain/payment"
)

func toPaymentResponse(p *payment.Payment) *dto.PaymentResponse {
	return &dto.PaymentResponse{
		ID:        p.ID,
		Amount:    p.Money.Amount,
		Currency:  p.Money.Currency,
		Status:    string(p.Status),
		CreatedAt: p.CreatedAt,
	}
}
