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
// Authorize deve retornar erro apenas em falhas transitórias/de comunicação
// (timeout, provedor indisponível). Uma recusa de negócio NÃO é erro: vem como
// AuthorizationResult{Outcome: OutcomeDeclined}.
type Gateway interface {
	Authorize(ctx context.Context, p *payment.Payment) (AuthorizationResult, error)
}
