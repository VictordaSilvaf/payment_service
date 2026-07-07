package usecase

import (
	"context"

	"payment_service/internal/application/dto"
	"payment_service/internal/domain/outbox"
	"payment_service/internal/domain/payment"
	"payment_service/internal/domain/psp"
)

// CapturePayment liquida um pagamento previamente autorizado (fluxo de captura
// manual). É uma ação síncrona iniciada pelo lojista via API: fala com o PSP e,
// em caso de sucesso, transiciona authorized → completed emitindo payment.completed
// (na mesma transação, via Outbox).
type CapturePayment struct {
	repo    payment.Repository
	gateway psp.Gateway
	outbox  outbox.Repository
	tx      TxManager
}

func NewCapturePayment(
	repo payment.Repository,
	gateway psp.Gateway,
	outboxRepo outbox.Repository,
	tx TxManager,
) *CapturePayment {
	return &CapturePayment{repo: repo, gateway: gateway, outbox: outboxRepo, tx: tx}
}

func (uc *CapturePayment) Execute(ctx context.Context, paymentID string) (*dto.PaymentResponse, error) {
	p, err := uc.repo.FindByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	// Transição de estado primeiro: rejeita capturas inválidas (ex.: pagamento não
	// autorizado) antes de contatar o PSP. A mutação só é persistida no fim.
	if err := p.Capture(); err != nil {
		return nil, err
	}

	// Captura no PSP. Erro aqui é transitório: nada é persistido e o lojista pode
	// tentar de novo (o pagamento continua authorized no banco).
	if _, err := uc.gateway.Capture(ctx, p); err != nil {
		return nil, err
	}

	event, err := buildPaymentEvent(p, eventPaymentCompleted)
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

func (uc *CapturePayment) withinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if uc.tx == nil {
		return fn(ctx)
	}
	return uc.tx.WithinTx(ctx, fn)
}

func (uc *CapturePayment) addEvent(ctx context.Context, event outbox.Event) error {
	if uc.outbox == nil {
		return nil
	}
	return uc.outbox.Add(ctx, event)
}
