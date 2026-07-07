package outbox

import "context"

// Repository is a driven port for the outbox table.
type Repository interface {
	// Add grava um evento pendente. Deve participar da transação ativa no
	// contexto (ver TxManager), para ser atômico com a escrita do agregado.
	Add(ctx context.Context, event Event) error
	// FetchUnpublished retorna até `limit` eventos ainda não publicados,
	// ordenados por data de criação (mais antigos primeiro).
	FetchUnpublished(ctx context.Context, limit int) ([]Event, error)
	// MarkPublished marca o evento como publicado, removendo-o da fila de pendentes.
	MarkPublished(ctx context.Context, id string) error
}
