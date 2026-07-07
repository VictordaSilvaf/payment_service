package usecase

import (
	"context"
	"encoding/json"
	"log"

	"payment_service/internal/domain/outbox"
	"payment_service/internal/domain/payment"
	"payment_service/internal/domain/psp"
)

// Tipos/rotas dos eventos emitidos após a autorização no PSP. O relay usa como
// routing key; o webhook service liga a fila a essas chaves.
const (
	eventPaymentCompleted = "payment.completed"
	eventPaymentFailed    = "payment.failed"
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
	repo    payment.Repository
	gateway psp.Gateway
	outbox  outbox.Repository
	tx      TxManager
}

// NewProcessPayment recebe o repositório de pagamentos, o gateway do PSP e,
// opcionalmente, o outbox e o TxManager. Quando presentes, a transição de estado
// do pagamento e o evento resultante (payment.completed/failed) são gravados na
// mesma transação (Outbox Pattern). Um gateway nil aprova por padrão (útil em
// testes simples).
func NewProcessPayment(
	repo payment.Repository,
	gateway psp.Gateway,
	outboxRepo outbox.Repository,
	tx TxManager,
) *ProcessPayment {
	return &ProcessPayment{repo: repo, gateway: gateway, outbox: outboxRepo, tx: tx}
}

func (uc *ProcessPayment) Execute(ctx context.Context, input ProcessPaymentInput) (ProcessPaymentOutput, error) {
	p, err := uc.repo.FindByID(ctx, input.PaymentID)
	if err != nil {
		return ProcessPaymentOutput{}, err
	}

	// Autoriza no PSP. Erro aqui é transitório (timeout/indisponível): o consumer
	// devolve a mensagem à fila e o pagamento permanece pending para nova tentativa.
	result, err := uc.authorize(ctx, p)
	if err != nil {
		return ProcessPaymentOutput{}, err
	}

	eventType, err := uc.applyOutcome(p, result)
	if err != nil {
		return ProcessPaymentOutput{}, err
	}

	event, err := buildPaymentEvent(p, eventType)
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

// authorize consulta o PSP. Sem gateway configurado (ex.: testes simples), aprova.
func (uc *ProcessPayment) authorize(ctx context.Context, p *payment.Payment) (psp.AuthorizationResult, error) {
	if uc.gateway == nil {
		return psp.AuthorizationResult{Outcome: psp.OutcomeApproved}, nil
	}
	return uc.gateway.Authorize(ctx, p)
}

// applyOutcome traduz o resultado do PSP em transição de estado do pagamento e
// devolve o tipo de evento correspondente.
func (uc *ProcessPayment) applyOutcome(p *payment.Payment, result psp.AuthorizationResult) (string, error) {
	if result.Outcome == psp.OutcomeApproved {
		if err := p.Complete(); err != nil {
			return "", err
		}
		return eventPaymentCompleted, nil
	}

	if err := p.Fail(); err != nil {
		return "", err
	}
	log.Printf("payment %s declined by psp: %s", p.ID, result.Reason)
	return eventPaymentFailed, nil
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

func buildPaymentEvent(p *payment.Payment, eventType string) (outbox.Event, error) {
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
	return outbox.NewEvent(p.ID, eventType, payload), nil
}
