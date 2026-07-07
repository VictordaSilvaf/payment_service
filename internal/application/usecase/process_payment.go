package usecase

import (
	"context"
	"encoding/json"

	"payment_service/internal/domain/outbox"
	"payment_service/internal/domain/payment"
)

// eventPaymentCompleted é o tipo/rota do evento emitido quando o pagamento é
// concluído; o webhook service liga a fila a essa chave para notificar lojistas.
const eventPaymentCompleted = "payment.completed"

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
	repo   payment.Repository
	outbox outbox.Repository
	tx     TxManager
}

// NewProcessPayment recebe o repositório de pagamentos e, opcionalmente, o outbox
// e o TxManager. Quando presentes, a conclusão do pagamento e o evento
// payment.completed são gravados na mesma transação (Outbox Pattern). Quando nil
// (ex.: testes simples), apenas o pagamento é atualizado.
func NewProcessPayment(repo payment.Repository, outboxRepo outbox.Repository, tx TxManager) *ProcessPayment {
	return &ProcessPayment{repo: repo, outbox: outboxRepo, tx: tx}
}

func (uc *ProcessPayment) Execute(ctx context.Context, input ProcessPaymentInput) (ProcessPaymentOutput, error) {
	p, err := uc.repo.FindByID(ctx, input.PaymentID)
	if err != nil {
		return ProcessPaymentOutput{}, err
	}

	if err := p.Complete(); err != nil {
		return ProcessPaymentOutput{}, err
	}

	event, err := buildCompletedEvent(p)
	if err != nil {
		return ProcessPaymentOutput{}, err
	}

	if err := uc.withinTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.Update(ctx, p); err != nil {
			return err
		}
		return uc.addEvent(ctx, event)
	}); err != nil {
		return ProcessPaymentOutput{}, err
	}

	return ProcessPaymentOutput{
		PaymentID: p.ID,
		Status:    string(p.Status),
	}, nil
}

// withinTx usa o TxManager quando presente; se nil, executa a função diretamente.
func (uc *ProcessPayment) withinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if uc.tx == nil {
		return fn(ctx)
	}
	return uc.tx.WithinTx(ctx, fn)
}

// addEvent grava o evento no outbox quando ele está configurado.
func (uc *ProcessPayment) addEvent(ctx context.Context, event outbox.Event) error {
	if uc.outbox == nil {
		return nil
	}
	return uc.outbox.Add(ctx, event)
}

func buildCompletedEvent(p *payment.Payment) (outbox.Event, error) {
	payload, err := json.Marshal(paymentCreatedPayload{
		ID:        p.ID,
		Amount:    p.Money.Amount,
		Currency:  p.Money.Currency,
		Status:    string(p.Status),
		CreatedAt: p.CreatedAt,
	})
	if err != nil {
		return outbox.Event{}, err
	}
	return outbox.NewEvent(p.ID, eventPaymentCompleted, payload), nil
}
