package webhook

import (
	"time"

	"github.com/google/uuid"
)

type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
)

// Delivery registra a tentativa de entrega de um evento a uma assinatura. Serve
// para auditoria e é a base para reprocessar entregas que falharam (roadmap Retry).
type Delivery struct {
	ID             string
	SubscriptionID string
	EventID        string // id estável do evento para o lojista deduplicar
	Status         DeliveryStatus
	Attempts       int
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewDelivery cria uma entrega pendente (ainda não tentada).
func NewDelivery(subscriptionID, eventID string) *Delivery {
	now := time.Now().UTC()
	return &Delivery{
		ID:             uuid.NewString(),
		SubscriptionID: subscriptionID,
		EventID:        eventID,
		Status:         DeliveryPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// MarkDelivered marca a entrega como bem-sucedida e limpa o último erro.
func (d *Delivery) MarkDelivered() {
	d.Status = DeliveryDelivered
	d.Attempts++
	d.LastError = ""
	d.UpdatedAt = time.Now().UTC()
}

// Fail marca a entrega como falha, guardando o motivo para diagnóstico.
func (d *Delivery) Fail(reason string) {
	d.Status = DeliveryFailed
	d.Attempts++
	d.LastError = reason
	d.UpdatedAt = time.Now().UTC()
}
