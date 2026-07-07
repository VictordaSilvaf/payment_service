package notification

import (
	"context"
	"log"

	domain "payment_service/internal/domain/notification"
)

// LogNotifier é uma implementação mock da porta Notifier: em vez de falar com um
// provedor real de e-mail/SMS/push, apenas registra a notificação no log. Serve
// para desenvolvimento e testes, sem depender de credenciais ou rede.
type LogNotifier struct{}

func NewLogNotifier() *LogNotifier {
	return &LogNotifier{}
}

func (n *LogNotifier) Send(_ context.Context, notif *domain.Notification) error {
	log.Printf(
		"📣 notificação [%s] para %s (pagamento %s): %s",
		notif.Channel, notif.Recipient, notif.PaymentID, notif.Message,
	)
	return nil
}
