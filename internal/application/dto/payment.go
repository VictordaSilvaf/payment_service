package dto

import "time"

type CreatePaymentRequest struct {
	Amount   int64  `json:"amount" binding:"required,gt=0"`
	Currency string `json:"currency" binding:"required"`
	// Installments é o número de parcelas (1 = à vista). Opcional; padrão 1.
	Installments int `json:"installments" binding:"omitempty,gte=1,lte=12"`
	// CaptureMethod define quando capturar: automatic (padrão) ou manual.
	CaptureMethod string `json:"capture_method" binding:"omitempty,oneof=automatic manual"`
}

// RefundPaymentRequest pede o estorno de um pagamento. Amount é opcional: quando
// ausente/zero, estorna todo o saldo restante (estorno total).
type RefundPaymentRequest struct {
	Amount int64 `json:"amount" binding:"omitempty,gt=0"`
}

type PaymentResponse struct {
	ID             string    `json:"id"`
	Amount         int64     `json:"amount"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"`
	CaptureMethod  string    `json:"capture_method"`
	Installments   int       `json:"installments"`
	RefundedAmount int64     `json:"refunded_amount"`
	CreatedAt      time.Time `json:"created_at"`
}
