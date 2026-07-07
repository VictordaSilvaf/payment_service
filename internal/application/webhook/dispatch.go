package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	domain "payment_service/internal/domain/webhook"
)

// DispatchWebhook entrega um evento a todas as assinaturas ativas do seu tipo.
// É executado pelo consumer do RabbitMQ a cada evento recebido.
type DispatchWebhook struct {
	subs       domain.SubscriptionRepository
	deliveries domain.DeliveryRepository
	sender     domain.Sender
}

func NewDispatchWebhook(
	subs domain.SubscriptionRepository,
	deliveries domain.DeliveryRepository,
	sender domain.Sender,
) *DispatchWebhook {
	return &DispatchWebhook{subs: subs, deliveries: deliveries, sender: sender}
}

// Execute busca as assinaturas do tipo do evento e tenta entregar a cada uma.
//
// Erros de infraestrutura (buscar assinaturas / salvar entrega) são retornados
// para que o consumer devolva a mensagem à fila (nack requeue). Já falhas HTTP de
// um endpoint específico são registradas como entrega "failed" e NÃO derrubam o
// lote — a reentrega de falhas fica a cargo do item de roadmap "Retry".
func (uc *DispatchWebhook) Execute(ctx context.Context, eventType string, payload []byte) error {
	subs, err := uc.subs.FindActiveByEventType(ctx, eventType)
	if err != nil {
		return fmt.Errorf("find subscriptions: %w", err)
	}

	aggregateID := extractAggregateID(payload)

	for _, sub := range subs {
		webhookID := deliveryID(sub.ID, eventType, aggregateID)
		delivery := domain.NewDelivery(sub.ID, webhookID)

		status, sendErr := uc.sender.Send(ctx, domain.SendRequest{
			URL:       sub.URL,
			EventType: eventType,
			WebhookID: webhookID,
			Signature: domain.Sign(sub.Secret, payload),
			Body:      payload,
		})

		switch {
		case sendErr != nil:
			delivery.Fail(sendErr.Error())
		case status < 200 || status >= 300:
			delivery.Fail(fmt.Sprintf("unexpected status code: %d", status))
		default:
			delivery.MarkDelivered()
		}

		if err := uc.deliveries.Save(ctx, delivery); err != nil {
			return fmt.Errorf("save delivery: %w", err)
		}
	}

	return nil
}

// deliveryID gera um id determinístico por (assinatura, evento). Como não muda
// entre reentregas, o lojista pode usá-lo para deduplicar (idempotência).
func deliveryID(subscriptionID, eventType, aggregateID string) string {
	sum := sha256.Sum256([]byte(subscriptionID + ":" + eventType + ":" + aggregateID))
	return hex.EncodeToString(sum[:])
}

// extractAggregateID lê o campo "id" do payload (id do pagamento) para compor o
// id determinístico da entrega. Se não conseguir, retorna string vazia.
func extractAggregateID(payload []byte) string {
	var envelope struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(payload, &envelope)
	return envelope.ID
}
