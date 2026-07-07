package audit

import "context"

// Repository é a porta de persistência da trilha de auditoria. A trilha é
// append-only e imutável: não expõe update nem delete.
type Repository interface {
	// Append grava um registro. É idempotente: um id já existente é ignorado, de
	// modo que a reentrega do mesmo evento não duplica nem altera a trilha.
	Append(ctx context.Context, entry *AuditEntry) error
}
