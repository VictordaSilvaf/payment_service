package psp

import (
	"context"
	"time"

	"github.com/google/uuid"

	"payment_service/internal/domain/payment"
	domainpsp "payment_service/internal/domain/psp"
)

// MockGateway simula um PSP externo para desenvolvimento e testes, sem depender
// de um provedor real (sem credenciais, custo ou rede). Implementa a porta
// domainpsp.Gateway.
//
// Regra determinística: valores pares são aprovados e ímpares recusados — assim
// é trivial exercitar ambos os caminhos (ex.: amount=1000 aprova, amount=1001
// recusa). A latência opcional simula o tempo de uma chamada de rede ao provedor.
type MockGateway struct {
	latency time.Duration
}

func NewMockGateway(latency time.Duration) *MockGateway {
	return &MockGateway{latency: latency}
}

func (g *MockGateway) Authorize(ctx context.Context, p *payment.Payment) (domainpsp.AuthorizationResult, error) {
	// Simula a latência da chamada ao provedor, respeitando o cancelamento do contexto.
	if g.latency > 0 {
		select {
		case <-time.After(g.latency):
		case <-ctx.Done():
			return domainpsp.AuthorizationResult{}, ctx.Err()
		}
	}

	providerID := "psp_" + uuid.NewString()

	if p.Money.Amount%2 == 0 {
		return domainpsp.AuthorizationResult{
			Outcome:    domainpsp.OutcomeApproved,
			ProviderID: providerID,
		}, nil
	}

	return domainpsp.AuthorizationResult{
		Outcome:    domainpsp.OutcomeDeclined,
		ProviderID: providerID,
		Reason:     "insufficient funds",
	}, nil
}
