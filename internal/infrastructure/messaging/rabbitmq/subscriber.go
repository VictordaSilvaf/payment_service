package rabbitmq

import (
	"context"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"payment_service/internal/infrastructure/config"
)

// MessageHandler processa uma mensagem recebida. Recebe a routing key (tipo do
// evento) e o corpo cru. Retornar erro faz o Subscriber devolver a mensagem à
// fila (nack requeue) para nova tentativa; retornar nil confirma (ack).
type MessageHandler func(ctx context.Context, routingKey string, body []byte) error

// Subscriber é um consumer genérico: declara a exchange, uma fila própria e a
// liga a uma ou mais routing keys, entregando cada mensagem ao handler. É usado
// pelo Webhook Service para consumir eventos como "payment.completed".
type Subscriber struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
	handler MessageHandler
}

func NewSubscriber(
	cfg config.RabbitMQConfig,
	queue string,
	bindingKeys []string,
	handler MessageHandler,
) (*Subscriber, error) {
	conn, err := amqp.Dial(cfg.URL())
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	// Declara a exchange (idempotente): durable=true, autoDelete=false.
	if err := ch.ExchangeDeclare(cfg.Exchange, exchangeType, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	// Fila própria e durável para este serviço, isolada da fila de pagamentos.
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	// Liga a fila a cada routing key de interesse (ex.: "payment.completed").
	for _, key := range bindingKeys {
		if err := ch.QueueBind(queue, key, cfg.Exchange, false, nil); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("bind queue %q: %w", key, err)
		}
	}

	// QoS: no máximo `prefetchCount` mensagens não confirmadas por vez.
	if err := ch.Qos(prefetchCount, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}

	return &Subscriber{conn: conn, channel: ch, queue: queue, handler: handler}, nil
}

// Start consome e bloqueia até o contexto ser cancelado ou o channel cair.
func (s *Subscriber) Start(ctx context.Context) error {
	deliveries, err := s.channel.Consume(s.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consuming: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("rabbitmq channel closed")
			}
			s.handle(ctx, msg)
		}
	}
}

func (s *Subscriber) handle(ctx context.Context, msg amqp.Delivery) {
	if err := s.handler(ctx, msg.RoutingKey, msg.Body); err != nil {
		// Erro de infraestrutura: devolve à fila para nova tentativa.
		log.Printf("handler error (routing key %s), requeueing: %v", msg.RoutingKey, err)
		_ = msg.Nack(false, true)
		return
	}
	_ = msg.Ack(false)
}

// Close encerra channel e conexão. Deve ser chamado no shutdown (defer no main).
func (s *Subscriber) Close() error {
	if s.channel != nil {
		s.channel.Close()
	}
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
