package audit

import "time"

// AggregateType identifica o tipo de agregado auditado. Hoje há apenas pagamentos,
// mas o campo deixa a trilha pronta para outros agregados no futuro.
type AggregateType string

const AggregatePayment AggregateType = "payment"

// AuditEntry é um registro imutável (append-only) da trilha de auditoria: guarda o
// evento que ocorreu, o agregado afetado e um retrato (snapshot) do payload como
// ele trafegou no broker. Uma vez gravado, nunca é alterado nem removido.
type AuditEntry struct {
	ID            string        // id determinístico do evento (dedup de reentregas)
	AggregateType AggregateType // ex.: "payment"
	AggregateID   string        // id do agregado (ex.: id do pagamento)
	EventType     string        // ex.: "payment.completed"
	Payload       []byte        // snapshot cru do evento
	RecordedAt    time.Time     // quando o registro entrou na trilha
}

// NewAuditEntry cria um registro pronto para ser anexado à trilha.
func NewAuditEntry(id string, aggregateType AggregateType, aggregateID, eventType string, payload []byte) *AuditEntry {
	return &AuditEntry{
		ID:            id,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		RecordedAt:    time.Now().UTC(),
	}
}
