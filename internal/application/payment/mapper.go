package payment

import (
	"payment_service/internal/application/dto"
	domainpayment "payment_service/internal/domain/payment"
)

func ToDomain(request dto.CreatePaymentRequest) (*domainpayment.Payment, error) {
	return domainpayment.New(request.Amount, request.Currency)
}

func ToResponse(p *domainpayment.Payment) *dto.PaymentResponse {
	return &dto.PaymentResponse{
		ID:        p.ID,
		Amount:    p.Money.Amount,
		Currency:  p.Money.Currency,
		Status:    string(p.Status),
		CreatedAt: p.CreatedAt,
	}
}

func ToResponses(payments []*domainpayment.Payment) []*dto.PaymentResponse {
	responses := make([]*dto.PaymentResponse, len(payments))
	for i, p := range payments {
		responses[i] = ToResponse(p)
	}
	return responses
}
