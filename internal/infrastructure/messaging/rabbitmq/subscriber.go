package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"payment_service/internal/infrastructure/config"
)

// Message é a mensagem entregue ao handler: id (MessageId da AMQP, usado para
// deduplicação), a routing key (tipo do evento) e o corpo cru.
type Message struct {
	ID         string
	RoutingKey string
	Body       []byte
}

// MessageHandler processa uma mensagem recebida. Retornar erro faz o Subscriber
// devolver a mensagem à fila (nack requeue) para nova tentativa; retornar nil
// confirma (ack).
type MessageHandler func(ctx context.Context, msg Message) error

// Subscriber é um consumer genérico: declara a exchange, uma fila própria e a
// liga a uma ou mais routing keys, entregando cada mensagem ao handler. É usado
// pelo Webhook Service para consumir eventos como "payment.completed".
type Subscriber struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	queue      string
	maxRetries int
	retryDelay time.Duration
	handler    MessageHandler
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

	// Declara a DLX + DLQ deste serviço e obtém os argumentos de dead-lettering.
	dlqArgs, err := declareDLQ(ch, cfg.Exchange, queue)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// Fila própria e durável para este serviço, isolada da fila de pagamentos,
	// já apontando para a sua DLQ via dlqArgs.
	if _, err := ch.QueueDeclare(queue, true, false, false, false, dlqArgs); err != nil {
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

	return &Subscriber{
		conn:       conn,
		channel:    ch,
		queue:      queue,
		maxRetries: cfg.MaxRetries,
		retryDelay: cfg.RetryDelay,
		handler:    handler,
	}, nil
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
	// Tenta o handler algumas vezes (erros de infraestrutura costumam ser transitórios).
	err := processWithRetry(ctx, s.maxRetries, s.retryDelay, func() error {
		return s.handler(ctx, Message{
			ID:         msg.MessageId,
			RoutingKey: msg.RoutingKey,
			Body:       msg.Body,
		})
	})
	if err != nil {
		// Esgotou as tentativas: Nack com requeue=false encaminha para a DLQ,
		// evitando o loop infinito do requeue=true.
		log.Printf("handler error (routing key %s) after %d retries, sending to DLQ: %v", msg.RoutingKey, s.maxRetries, err)
		_ = msg.Nack(false, false)
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
