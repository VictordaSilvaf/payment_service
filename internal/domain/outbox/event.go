package outbox

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event é uma mensagem pendente de publicação, gravada na mesma transação do
// agregado que a originou (ex.: um pagamento). É o coração do Outbox Pattern:
// garante que persistir o dado e registrar o evento sejam atômicos.
type Event struct {
	ID          string
	AggregateID string          // id do agregado de origem (ex.: payment.ID)
	Type        string          // tipo/rota do evento (ex.: "payment.created")
	Payload     json.RawMessage // corpo serializado que será publicado no broker
	CreatedAt   time.Time
	PublishedAt *time.Time // nil enquanto não publicado
}

// NewEvent cria um evento pendente (ainda não publicado) pronto para ser gravado.
func NewEvent(aggregateID, eventType string, payload json.RawMessage) Event {
	return Event{
		ID:          uuid.New().String(),
		AggregateID: aggregateID,
		Type:        eventType,
		Payload:     payload,
		CreatedAt:   time.Now().UTC(),
	}
}
