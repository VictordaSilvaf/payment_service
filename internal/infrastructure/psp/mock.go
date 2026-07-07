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
	if err := g.simulateLatency(ctx); err != nil {
		return domainpsp.AuthorizationResult{}, err
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

// Capture liquida uma autorização prévia. No mock a captura sempre é aceita
// (a decisão de negócio interessante fica na autorização); devolve o id da
// operação no provedor.
func (g *MockGateway) Capture(ctx context.Context, _ *payment.Payment) (string, error) {
	if err := g.simulateLatency(ctx); err != nil {
		return "", err
	}
	return "cap_" + uuid.NewString(), nil
}

// Refund devolve, total ou parcialmente, um pagamento capturado. No mock sempre
// é aceito; devolve o id do estorno no provedor.
func (g *MockGateway) Refund(ctx context.Context, _ *payment.Payment, _ int64) (string, error) {
	if err := g.simulateLatency(ctx); err != nil {
		return "", err
	}
	return "ref_" + uuid.NewString(), nil
}

// simulateLatency aguarda a latência configurada, respeitando o cancelamento do
// contexto (para simular o tempo de uma chamada de rede ao provedor).
func (g *MockGateway) simulateLatency(ctx context.Context) error {
	if g.latency <= 0 {
		return nil
	}
	select {
	case <-time.After(g.latency):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
