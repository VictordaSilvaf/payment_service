package payment

import "context"

// EventPublisher is a driven port for publishing domain events.
type EventPublisher interface {
	PublishCreated(ctx context.Context, p *Payment) error
}
