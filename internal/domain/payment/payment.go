package payment

import (
	"time"

	"github.com/google/uuid"
)

type Payment struct {
	ID        string
	Money     Money
	Status    Status
	CreatedAt time.Time
}

func New(amount int64, currency string) (*Payment, error) {
	money, err := NewMoney(amount, currency)
	if err != nil {
		return nil, err
	}

	return &Payment{
		ID:        uuid.New().String(),
		Money:     money,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (p *Payment) Complete() error {
	if !p.Status.IsValid() {
		return ErrInvalidStatus
	}
	p.Status = StatusCompleted
	return nil
}

func (p *Payment) Fail() error {
	if !p.Status.IsValid() {
		return ErrInvalidStatus
	}
	p.Status = StatusFailed
	return nil
}
