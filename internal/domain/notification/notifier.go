package notification

import "context"

// Notifier é a porta de saída (driven) para entregar a notificação ao usuário
// final. A implementação real falaria com um provedor de e-mail/SMS/push; o mock
// apenas registra a mensagem (log) para desenvolvimento e testes.
//
// Send deve retornar erro em falhas de envio (transitórias ou não). O chamador
// registra a falha e propaga o erro para que o Subscriber retente e, se persistir,
// encaminhe a mensagem à DLQ.
type Notifier interface {
	Send(ctx context.Context, n *Notification) error
}
