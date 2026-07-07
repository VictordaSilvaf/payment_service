package payment

import (
	"time"

	"github.com/google/uuid"
)

// maxInstallments limita o parcelamento (ex.: cartão de crédito) a 12x.
const maxInstallments = 12

type Payment struct {
	ID             string
	Money          Money
	Status         Status
	CaptureMethod  CaptureMethod
	Installments   int
	RefundedAmount int64 // total já estornado, em centavos
	CreatedAt      time.Time
}

// New cria um pagamento com captura automática à vista (1x) — o fluxo padrão.
func New(amount int64, currency string) (*Payment, error) {
	return NewWithOptions(amount, currency, 1, CaptureAutomatic)
}

// NewWithOptions cria um pagamento permitindo escolher o parcelamento e o método
// de captura (automático ou manual), validando cada um.
func NewWithOptions(amount int64, currency string, installments int, captureMethod CaptureMethod) (*Payment, error) {
	money, err := NewMoney(amount, currency)
	if err != nil {
		return nil, err
	}
	if installments < 1 || installments > maxInstallments {
		return nil, ErrInvalidInstallments
	}
	if !captureMethod.IsValid() {
		return nil, ErrInvalidCaptureMethod
	}

	return &Payment{
		ID:            uuid.New().String(),
		Money:         money,
		Status:        StatusPending,
		CaptureMethod: captureMethod,
		Installments:  installments,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// MarkAuthorized registra a autorização no PSP sem capturar (fluxo manual):
// pending → authorized.
func (p *Payment) MarkAuthorized() error {
	if p.Status != StatusPending {
		return ErrInvalidTransition
	}
	p.Status = StatusAuthorized
	return nil
}

// Complete captura o pagamento no fluxo automático: pending → completed.
func (p *Payment) Complete() error {
	if p.Status != StatusPending {
		return ErrInvalidTransition
	}
	p.Status = StatusCompleted
	return nil
}

// Capture liquida um pagamento previamente autorizado (fluxo manual):
// authorized → completed.
func (p *Payment) Capture() error {
	if p.Status != StatusAuthorized {
		return ErrInvalidTransition
	}
	p.Status = StatusCompleted
	return nil
}

// Fail marca a recusa do PSP: pending → failed.
func (p *Payment) Fail() error {
	if p.Status != StatusPending {
		return ErrInvalidTransition
	}
	p.Status = StatusFailed
	return nil
}

// Refund estorna um valor de um pagamento capturado. Aceita estornos parciais
// sucessivos até o total do pagamento; ao atingir o total, o status vira
// refunded, caso contrário partially_refunded.
func (p *Payment) Refund(amount int64) error {
	if p.Status != StatusCompleted && p.Status != StatusPartiallyRefunded {
		return ErrInvalidTransition
	}
	if amount <= 0 {
		return ErrInvalidRefundAmount
	}
	if amount > p.RefundableAmount() {
		return ErrRefundExceedsAmount
	}

	p.RefundedAmount += amount
	if p.RefundedAmount == p.Money.Amount {
		p.Status = StatusRefunded
	} else {
		p.Status = StatusPartiallyRefunded
	}
	return nil
}

// RefundableAmount é o quanto ainda pode ser estornado (total menos já estornado).
func (p *Payment) RefundableAmount() int64 {
	return p.Money.Amount - p.RefundedAmount
}
