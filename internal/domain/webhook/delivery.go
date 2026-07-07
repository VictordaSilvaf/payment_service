package webhook

import (
	"time"

	"github.com/google/uuid"
)

type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"    // falhou, ainda elegível a retry
	DeliveryExhausted DeliveryStatus = "exhausted" // estourou o máximo de tentativas (terminal)
)

// Delivery registra a tentativa de entrega de um evento a uma assinatura. Guarda
// o corpo e o tipo do evento para poder ser reenviada por um processo de retry
// sem depender da mensagem original do broker.
type Delivery struct {
	ID             string
	SubscriptionID string
	EventID        string // id estável do evento para o lojista deduplicar
	EventType      string
	Payload        []byte
	Status         DeliveryStatus
	Attempts       int
	LastError      string
	NextAttemptAt  time.Time // quando fica elegível ao próximo retry (status = failed)
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewDelivery cria uma entrega pendente (ainda não tentada).
func NewDelivery(subscriptionID, eventID, eventType string, payload []byte) *Delivery {
	now := time.Now().UTC()
	return &Delivery{
		ID:             uuid.NewString(),
		SubscriptionID: subscriptionID,
		EventID:        eventID,
		EventType:      eventType,
		Payload:        payload,
		Status:         DeliveryPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// MarkDelivered marca a entrega como bem-sucedida e limpa o agendamento de retry.
func (d *Delivery) MarkDelivered() {
	d.Status = DeliveryDelivered
	d.Attempts++
	d.LastError = ""
	d.NextAttemptAt = time.Time{}
	d.UpdatedAt = time.Now().UTC()
}

// MarkForRetry registra a falha e agenda a próxima tentativa (backoff).
func (d *Delivery) MarkForRetry(reason string, nextAttemptAt time.Time) {
	d.Status = DeliveryFailed
	d.Attempts++
	d.LastError = reason
	d.NextAttemptAt = nextAttemptAt
	d.UpdatedAt = time.Now().UTC()
}

// MarkExhausted encerra a entrega após esgotar as tentativas (não será mais retentada).
func (d *Delivery) MarkExhausted(reason string) {
	d.Status = DeliveryExhausted
	d.Attempts++
	d.LastError = reason
	d.NextAttemptAt = time.Time{}
	d.UpdatedAt = time.Now().UTC()
}
