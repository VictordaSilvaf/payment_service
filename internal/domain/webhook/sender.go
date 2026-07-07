package webhook

import "context"

// SendRequest reúne tudo que o adapter HTTP precisa para entregar um webhook.
type SendRequest struct {
	URL       string
	EventType string
	WebhookID string
	Signature string
	Body      []byte
}

// Sender é a porta de saída (driven) para entregar o webhook ao endpoint externo.
// A implementação (HTTP) faz o POST e devolve o status code recebido.
type Sender interface {
	Send(ctx context.Context, req SendRequest) (int, error)
}
