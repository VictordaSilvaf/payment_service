package payment

import "errors"

var (
	ErrNotFound      = errors.New("payment not found")
	ErrInvalidAmount = errors.New("amount must be greater than zero")
	ErrInvalidStatus = errors.New("invalid payment status")
)
