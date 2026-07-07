package usecase

import (
	"context"

	"payment_service/internal/application/dto"
	"payment_service/internal/domain/outbox"
	"payment_service/internal/domain/payment"
	"payment_service/internal/domain/psp"
)

// RefundPayment estorna, total ou parcialmente, um pagamento capturado. É uma ação
// síncrona iniciada pelo lojista via API: fala com o PSP e, em caso de sucesso,
// atualiza o pagamento (refunded/partially_refunded) emitindo payment.refunded na
// mesma transação (via Outbox).
type RefundPayment struct {
	repo    payment.Repository
	gateway psp.Gateway
	outbox  outbox.Repository
	tx      TxManager
}

func NewRefundPayment(
	repo payment.Repository,
	gateway psp.Gateway,
	outboxRepo outbox.Repository,
	tx TxManager,
) *RefundPayment {
	return &RefundPayment{repo: repo, gateway: gateway, outbox: outboxRepo, tx: tx}
}

// Execute estorna `amount` centavos do pagamento. Um amount <= 0 estorna o saldo
// restante por completo (estorno integral do que ainda não foi devolvido).
func (uc *RefundPayment) Execute(ctx context.Context, paymentID string, amount int64) (*dto.PaymentResponse, error) {
	p, err := uc.repo.FindByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	if amount <= 0 {
		amount = p.RefundableAmount()
	}

	// Aplica o estorno no domínio primeiro: valida o estado e os limites (não pode
	// exceder o saldo) antes de contatar o PSP. A mutação só é persistida no fim.
	if err := p.Refund(amount); err != nil {
		return nil, err
	}

	// Estorno no PSP. Erro aqui é transitório: nada é persistido e o lojista pode
	// tentar de novo (o pagamento mantém o estado anterior no banco).
	if _, err := uc.gateway.Refund(ctx, p, amount); err != nil {
		return nil, err
	}

	event, err := buildPaymentEvent(p, eventPaymentRefunded)
	if err != nil {
		return nil, err
	}

	if err := uc.withinTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.Update(ctx, p); err != nil {
			return err
		}
		return uc.addEvent(ctx, event)
	}); err != nil {
		return nil, err
	}

	return toPaymentResponse(p), nil
}

func (uc *RefundPayment) withinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if uc.tx == nil {
		return fn(ctx)
	}
	return uc.tx.WithinTx(ctx, fn)
}

func (uc *RefundPayment) addEvent(ctx context.Context, event outbox.Event) error {
	if uc.outbox == nil {
		return nil
	}
	return uc.outbox.Add(ctx, event)
}
