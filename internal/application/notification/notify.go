package notification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	domain "payment_service/internal/domain/notification"
)

// NotifyPayment consome um evento de pagamento e envia uma notificação ao usuário
// final. É executado pelo Notification Service a cada evento recebido.
type NotifyPayment struct {
	notifier domain.Notifier
	repo     domain.Repository
	channel  domain.Channel
}

func NewNotifyPayment(notifier domain.Notifier, repo domain.Repository, channel domain.Channel) *NotifyPayment {
	if channel == "" {
		channel = domain.ChannelEmail
	}
	return &NotifyPayment{notifier: notifier, repo: repo, channel: channel}
}

// paymentEventPayload é o formato do payload dos eventos payment.completed/failed.
type paymentEventPayload struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

// Execute monta a notificação a partir do evento, tenta enviá-la e grava o
// resultado. Uma falha de envio é registrada como "failed" e o erro é propagado,
// para que o Subscriber retente e, se persistir, encaminhe a mensagem à DLQ.
func (uc *NotifyPayment) Execute(ctx context.Context, eventType string, payload []byte) error {
	var evt paymentEventPayload
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("unmarshal payment event: %w", err)
	}

	n := domain.NewNotification(
		notificationID(evt.ID, eventType, uc.channel),
		evt.ID,
		eventType,
		uc.channel,
		recipientFor(evt.ID),
		buildMessage(eventType, evt),
	)

	sendErr := uc.notifier.Send(ctx, n)
	if sendErr != nil {
		n.MarkFailed(sendErr.Error())
	} else {
		n.MarkSent()
	}

	if err := uc.repo.Save(ctx, n); err != nil {
		return fmt.Errorf("save notification: %w", err)
	}

	if sendErr != nil {
		return fmt.Errorf("send notification: %w", sendErr)
	}
	return nil
}

// notificationID gera um id determinístico por (pagamento, evento, canal). Como
// não muda entre reentregas, o upsert evita notificar o usuário em duplicidade.
func notificationID(paymentID, eventType string, channel domain.Channel) string {
	sum := sha256.Sum256([]byte(paymentID + ":" + eventType + ":" + string(channel)))
	return hex.EncodeToString(sum[:])
}

// recipientFor deriva um destinatário de exemplo a partir do id do pagamento.
// Em um sistema real viria do cadastro do cliente; aqui é um placeholder do mock.
func recipientFor(paymentID string) string {
	return fmt.Sprintf("customer+%s@example.com", paymentID)
}

func buildMessage(eventType string, evt paymentEventPayload) string {
	amount := formatAmount(evt.Amount, evt.Currency)
	switch eventType {
	case "payment.completed":
		return fmt.Sprintf("Seu pagamento de %s foi aprovado. ✅", amount)
	case "payment.failed":
		return fmt.Sprintf("Seu pagamento de %s foi recusado. ❌", amount)
	default:
		return fmt.Sprintf("Atualização do seu pagamento de %s: %s.", amount, evt.Status)
	}
}

// formatAmount converte o valor em centavos para uma representação legível.
func formatAmount(amountCents int64, currency string) string {
	return fmt.Sprintf("%s %.2f", currency, float64(amountCents)/100)
}
