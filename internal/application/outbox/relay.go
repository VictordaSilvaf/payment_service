package outbox

import (
	"context"
	"fmt"
	"log"
	"time"

	domainoutbox "payment_service/internal/domain/outbox"
)

// Publisher é a porta de saída que o relay usa para publicar o evento no broker.
// A implementação (RabbitMQ) publica o payload cru na exchange usando a routing key.
// O messageID (id do evento no outbox) trafega como identificador da mensagem para
// que consumidores possam deduplicar reentregas (ex.: a trilha de auditoria).
type Publisher interface {
	Publish(ctx context.Context, routingKey, messageID string, body []byte) error
}

// Relay é o dispatcher do Outbox Pattern: periodicamente lê eventos pendentes,
// publica no broker e os marca como publicados. Entrega at-least-once.
type Relay struct {
	repo      domainoutbox.Repository
	publisher Publisher
	interval  time.Duration
	batchSize int
}

func NewRelay(repo domainoutbox.Repository, publisher Publisher, interval time.Duration, batchSize int) *Relay {
	return &Relay{
		repo:      repo,
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
	}
}

// Run bloqueia executando o dispatch a cada `interval`, até o contexto ser cancelado.
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.dispatch(ctx); err != nil {
				// Não interrompe o loop: erros transitórios (broker fora do ar)
				// serão retentados no próximo tick, pois o evento continua pendente.
				log.Printf("outbox dispatch error: %v", err)
			}
		}
	}
}

// dispatch publica um lote de eventos pendentes, um a um, marcando cada sucesso.
// Se a publicação falhar, para o lote: o evento não é marcado e será retentado.
func (r *Relay) dispatch(ctx context.Context) error {
	events, err := r.repo.FetchUnpublished(ctx, r.batchSize)
	if err != nil {
		return fmt.Errorf("fetch unpublished: %w", err)
	}

	for _, event := range events {
		if err := r.publisher.Publish(ctx, event.Type, event.ID, event.Payload); err != nil {
			return fmt.Errorf("publish event %s: %w", event.ID, err)
		}
		if err := r.repo.MarkPublished(ctx, event.ID); err != nil {
			return fmt.Errorf("mark published %s: %w", event.ID, err)
		}
		log.Printf("outbox event %s (%s) published", event.ID, event.Type)
	}
	return nil
}
