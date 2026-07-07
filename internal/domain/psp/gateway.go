package psp

import (
	"context"

	"payment_service/internal/domain/payment"
)

type Outcome string

const (
	OutcomeApproved Outcome = "approved"
	OutcomeDeclined Outcome = "declined"
)

// AuthorizationResult é a resposta do PSP a uma tentativa de autorização.
type AuthorizationResult struct {
	Outcome    Outcome
	ProviderID string // id da transação no provedor (ex.: "psp_...")
	Reason     string // motivo da recusa, quando Outcome == OutcomeDeclined
}

// Gateway é a porta de saída (driven) para o provedor de pagamentos (PSP).
// A implementação real falaria com Stripe/Adyen/etc.; o mock simula o
// comportamento (aprovação, recusa, latência) para desenvolvimento e testes.
//
// Todos os métodos devem retornar erro apenas em falhas transitórias/de
// comunicação (timeout, provedor indisponível). Uma recusa de negócio NÃO é erro:
// na autorização, vem como AuthorizationResult{Outcome: OutcomeDeclined}.
//
//   - Authorize reserva os fundos (fluxo automático captura em seguida).
//   - Capture liquida uma autorização prévia (fluxo de captura manual).
//   - Refund devolve, total ou parcialmente, um pagamento já capturado.
//
// Capture e Refund devolvem o id da operação no provedor (para conciliação).
type Gateway interface {
	Authorize(ctx context.Context, p *payment.Payment) (AuthorizationResult, error)
	Capture(ctx context.Context, p *payment.Payment) (string, error)
	Refund(ctx context.Context, p *payment.Payment, amount int64) (string, error)
}
