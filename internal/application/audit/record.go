package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	domain "payment_service/internal/domain/audit"
)

// RecordAudit registra na trilha de auditoria cada evento de pagamento recebido.
// É executado pelo Audit Service para todos os eventos "payment.*".
type RecordAudit struct {
	repo domain.Repository
}

func NewRecordAudit(repo domain.Repository) *RecordAudit {
	return &RecordAudit{repo: repo}
}

// aggregateRef extrai o id do agregado do payload. Todos os eventos de pagamento
// carregam ao menos o campo "id".
type aggregateRef struct {
	ID string `json:"id"`
}

// Execute grava o evento na trilha. O eventID (MessageId propagado pelo relay a
// partir do id do outbox) é a chave determinística: reentregas do mesmo evento não
// duplicam a trilha. Quando ausente, deriva uma chave estável do conteúdo.
func (uc *RecordAudit) Execute(ctx context.Context, eventID, eventType string, payload []byte) error {
	var ref aggregateRef
	if err := json.Unmarshal(payload, &ref); err != nil {
		return fmt.Errorf("unmarshal audit event: %w", err)
	}

	entry := domain.NewAuditEntry(
		auditID(eventID, eventType, payload),
		domain.AggregatePayment,
		ref.ID,
		eventType,
		payload,
	)

	if err := uc.repo.Append(ctx, entry); err != nil {
		return fmt.Errorf("append audit entry: %w", err)
	}
	return nil
}

// auditID devolve o id do evento quando presente; caso contrário, um hash estável
// do conteúdo (tipo + payload), garantindo dedup mesmo sem MessageId.
func auditID(eventID, eventType string, payload []byte) string {
	if eventID != "" {
		return eventID
	}
	sum := sha256.Sum256(append([]byte(eventType+":"), payload...))
	return hex.EncodeToString(sum[:])
}
