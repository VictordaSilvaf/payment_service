package payment

import "errors"

var (
	ErrNotFound             = errors.New("payment not found")
	ErrInvalidAmount        = errors.New("amount must be greater than zero")
	ErrInvalidStatus        = errors.New("invalid payment status")
	ErrInvalidInstallments  = errors.New("installments must be between 1 and 12")
	ErrInvalidCaptureMethod = errors.New("capture method must be automatic or manual")
	// ErrInvalidTransition indica uma operação incompatível com o estado atual do
	// pagamento (ex.: capturar um pagamento que não está autorizado).
	ErrInvalidTransition   = errors.New("invalid payment state transition")
	ErrInvalidRefundAmount = errors.New("refund amount must be greater than zero")
	ErrRefundExceedsAmount = errors.New("refund amount exceeds refundable balance")
)
