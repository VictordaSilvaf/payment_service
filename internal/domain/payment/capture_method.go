package payment

// CaptureMethod define quando os fundos autorizados são capturados (liquidados).
//
//   - automatic: o pagamento é capturado logo após a autorização (fluxo padrão).
//   - manual: a autorização apenas reserva os fundos; a captura é disparada depois
//     por uma chamada explícita (POST /payments/:id/capture). Útil para cenários
//     como reserva de hotel/aluguel de carro, onde o valor final só é conhecido na
//     entrega.
type CaptureMethod string

const (
	CaptureAutomatic CaptureMethod = "automatic"
	CaptureManual    CaptureMethod = "manual"
)

func (m CaptureMethod) IsValid() bool {
	switch m {
	case CaptureAutomatic, CaptureManual:
		return true
	default:
		return false
	}
}
