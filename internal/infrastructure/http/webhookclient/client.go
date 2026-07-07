package webhookclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"payment_service/internal/domain/webhook"
)

// Client entrega webhooks via HTTP POST. Implementa a porta domain.Sender.
//
// Observação de segurança: em produção, o endpoint informado pelo lojista deveria
// passar por uma verificação anti-SSRF (bloquear IPs privados/loopback/metadata)
// antes do POST. Aqui mantemos apenas o timeout e a validação de esquema feita no
// domínio; o hardening de SSRF é um próximo passo.
type Client struct {
	http *http.Client
}

func New(timeout time.Duration) *Client {
	return &Client{http: &http.Client{Timeout: timeout}}
}

func (c *Client) Send(ctx context.Context, req webhook.SendRequest) (int, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Webhook-Id", req.WebhookID)
	httpReq.Header.Set("X-Webhook-Event", req.EventType)
	httpReq.Header.Set("X-Webhook-Signature", req.Signature)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Drena o corpo para permitir o reuso da conexão (keep-alive).
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}
