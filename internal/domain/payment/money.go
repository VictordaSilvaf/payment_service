package payment

import "errors"

type Money struct {
	Amount   int64
	Currency string
}

func NewMoney(amount int64, currency string) (Money, error) {
	if amount <= 0 {
		return Money{}, ErrInvalidAmount
	}
	if currency == "" {
		return Money{}, errors.New("currency is required")
	}
	return Money{Amount: amount, Currency: currency}, nil
}
