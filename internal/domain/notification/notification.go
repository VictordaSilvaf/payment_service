package notification

import "time"

// Channel é o meio pelo qual a notificação é enviada ao usuário final.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"
	ChannelPush  Channel = "push"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
)

// Notification é o registro de uma notificação ao usuário final disparada por um
// evento de pagamento. Serve de log/auditoria e é a base para deduplicação
// (o mesmo evento não deve notificar o usuário duas vezes).
type Notification struct {
	ID        string // determinístico por (pagamento, evento, canal) → dedup
	PaymentID string
	EventType string
	Channel   Channel
	Recipient string
	Message   string
	Status    Status
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewNotification cria uma notificação pendente (ainda não enviada).
func NewNotification(id, paymentID, eventType string, channel Channel, recipient, message string) *Notification {
	now := time.Now().UTC()
	return &Notification{
		ID:        id,
		PaymentID: paymentID,
		EventType: eventType,
		Channel:   channel,
		Recipient: recipient,
		Message:   message,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// MarkSent marca a notificação como enviada com sucesso e limpa o último erro.
func (n *Notification) MarkSent() {
	n.Status = StatusSent
	n.LastError = ""
	n.UpdatedAt = time.Now().UTC()
}

// MarkFailed marca a notificação como falha, guardando o motivo para diagnóstico.
func (n *Notification) MarkFailed(reason string) {
	n.Status = StatusFailed
	n.LastError = reason
	n.UpdatedAt = time.Now().UTC()
}
