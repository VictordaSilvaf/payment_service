package usecase

import (
	"context"
	"encoding/json"
	"time"

	"payment_service/internal/application/dto"
	"payment_service/internal/domain/outbox"
	"payment_service/internal/domain/payment"
)

// eventPaymentCreated é o tipo/rota do evento; o relay usa como routing key e o
// consumer liga a fila a essa mesma chave.
const eventPaymentCreated = "payment.created"

// TxManager é uma porta de aplicação para executar uma unidade de trabalho de
// forma atômica. A implementação (Postgres) abre a transação e a propaga via
// contexto para os repositórios.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// paymentCreatedPayload é o formato serializado do evento gravado no outbox.
// As chaves precisam bater com o que o consumer espera.
type paymentCreatedPayload struct {
	ID        string    `json:"id"`
	Amount    int64     `json:"amount"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatePayment struct {
	repo   payment.Repository
	outbox outbox.Repository
	tx     TxManager
}

func NewCreatePayment(repo payment.Repository, outboxRepo outbox.Repository, tx TxManager) *CreatePayment {
	return &CreatePayment{repo: repo, outbox: outboxRepo, tx: tx}
}

// Execute cria o pagamento e registra o evento payment.created no outbox dentro
// de uma única transação (Outbox Pattern). A publicação no broker é feita depois,
// de forma assíncrona, pelo relay — eliminando o problema de dual-write.
func (uc *CreatePayment) Execute(ctx context.Context, req dto.CreatePaymentRequest) (*dto.PaymentResponse, error) {
	p, err := payment.New(req.Amount, req.Currency)
	if err != nil {
		return nil, err
	}

	event, err := buildCreatedEvent(p)
	if err != nil {
		return nil, err
	}

	if err := uc.withinTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.Save(ctx, p); err != nil {
			return err
		}
		return uc.outbox.Add(ctx, event)
	}); err != nil {
		return nil, err
	}

	return toPaymentResponse(p), nil
}

// withinTx usa o TxManager quando presente; se nil (ex.: testes simples), executa
// a função diretamente.
func (uc *CreatePayment) withinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if uc.tx == nil {
		return fn(ctx)
	}
	return uc.tx.WithinTx(ctx, fn)
}

func buildCreatedEvent(p *payment.Payment) (outbox.Event, error) {
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
	return outbox.NewEvent(p.ID, eventPaymentCreated, payload), nil
}
