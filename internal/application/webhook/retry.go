package webhook

import (
	"context"
	"fmt"
	"log"
	"time"

	domain "payment_service/internal/domain/webhook"
)

// BackoffPolicy define o número máximo de tentativas e o atraso base do backoff
// exponencial usado entre reentregas de webhook.
type BackoffPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

// nextDelay devolve o atraso antes da próxima tentativa, crescendo de forma
// exponencial: base, base*2, base*4, ... (attempt é o número da tentativa recém-feita).
func (p BackoffPolicy) nextDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	return p.BaseDelay << (attempt - 1)
}

// exhausted informa se, após `attempt` tentativas, não se deve tentar mais.
func (p BackoffPolicy) exhausted(attempt int) bool {
	return attempt >= p.MaxAttempts
}

// applyAttemptOutcome aplica o resultado de uma tentativa de entrega ao registro:
// sucesso → delivered; falha → agenda retry (com backoff) ou esgota se atingiu o
// máximo de tentativas. É compartilhada pela entrega inicial e pelo retry.
func applyAttemptOutcome(policy BackoffPolicy, d *domain.Delivery, status int, sendErr error, now time.Time) {
	if sendErr == nil && status >= 200 && status < 300 {
		d.MarkDelivered()
		return
	}

	reason := deliveryFailureReason(status, sendErr)

	// Attempts+1 é a tentativa que acabou de acontecer.
	if policy.exhausted(d.Attempts + 1) {
		d.MarkExhausted(reason)
		return
	}
	d.MarkForRetry(reason, now.Add(policy.nextDelay(d.Attempts+1)))
}

func deliveryFailureReason(status int, sendErr error) string {
	if sendErr != nil {
		return sendErr.Error()
	}
	return fmt.Sprintf("unexpected status code: %d", status)
}

// RetryDeliveries reenvia periodicamente as entregas que falharam e ainda estão
// dentro do limite de tentativas. Segue o mesmo padrão do relay do outbox: um
// poller sobre o banco, desacoplado do broker.
type RetryDeliveries struct {
	deliveries domain.DeliveryRepository
	sender     domain.Sender
	policy     BackoffPolicy
	interval   time.Duration
	batchSize  int
}

func NewRetryDeliveries(
	deliveries domain.DeliveryRepository,
	sender domain.Sender,
	policy BackoffPolicy,
	interval time.Duration,
	batchSize int,
) *RetryDeliveries {
	return &RetryDeliveries{
		deliveries: deliveries,
		sender:     sender,
		policy:     policy,
		interval:   interval,
		batchSize:  batchSize,
	}
}

// Run bloqueia executando o retry a cada `interval`, até o contexto ser cancelado.
func (uc *RetryDeliveries) Run(ctx context.Context) error {
	ticker := time.NewTicker(uc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := uc.retryBatch(ctx); err != nil {
				// Não interrompe o loop: erros transitórios são retentados no próximo tick.
				log.Printf("webhook retry error: %v", err)
			}
		}
	}
}

// retryBatch busca entregas elegíveis e tenta reenviar cada uma, persistindo o
// novo resultado (entregue, reagendada ou esgotada).
func (uc *RetryDeliveries) retryBatch(ctx context.Context) error {
	items, err := uc.deliveries.FetchRetriable(ctx, uc.batchSize, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("fetch retriable: %w", err)
	}

	for _, item := range items {
		d := item.Delivery

		status, sendErr := uc.sender.Send(ctx, domain.SendRequest{
			URL:       item.URL,
			EventType: d.EventType,
			WebhookID: d.EventID,
			Signature: domain.Sign(item.Secret, d.Payload),
			Body:      d.Payload,
		})

		applyAttemptOutcome(uc.policy, d, status, sendErr, time.Now().UTC())

		if err := uc.deliveries.Save(ctx, d); err != nil {
			return fmt.Errorf("save delivery %s: %w", d.ID, err)
		}
		log.Printf("webhook delivery %s retried: status=%s attempts=%d", d.ID, d.Status, d.Attempts)
	}

	return nil
}
