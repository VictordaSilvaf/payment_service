package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"payment_service/internal/domain/payment"
	"payment_service/internal/infrastructure/config"
)

const (
	// exchangeType "topic" permite roteamento por padrões de routing key.
	exchangeType = "topic"
	// routingKey identifica o tipo de evento; consumers ligam a fila a esta chave.
	routingKey = "payment.created"
)

// PaymentPublisher publica eventos de pagamento no RabbitMQ.
// É um adapter de saída (driven): implementa a porta EventPublisher do domínio.
type PaymentPublisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
}

// paymentCreatedEvent é o formato de transporte do evento (o que trafega no broker).
// Mantido separado da entidade de domínio para desacoplar persistência/mensageria do núcleo.
type paymentCreatedEvent struct {
	ID        string    `json:"id"`
	Amount    int64     `json:"amount"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// NewPaymentPublisher abre a conexão e garante que exchange, fila e binding existam.
// Declarar tudo aqui torna a inicialização idempotente e independente da ordem de subida.
func NewPaymentPublisher(cfg config.RabbitMQConfig) (*PaymentPublisher, error) {
	// Abre a conexão TCP com o broker.
	conn, err := amqp.Dial(cfg.URL())
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}

	// O channel é a sessão lógica onde todas as operações AMQP acontecem.
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	// Declara a exchange (idempotente). Parâmetros:
	// durable=true (sobrevive a restart do broker), autoDelete=false,
	// internal=false, noWait=false.
	if err := ch.ExchangeDeclare(
		cfg.Exchange,
		exchangeType,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	// Declara a fila (idempotente). Parâmetros:
	// durable=true, autoDelete=false, exclusive=false, noWait=false.
	if _, err := ch.QueueDeclare(
		cfg.Queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	// Liga a fila à exchange pela routing key, garantindo que os eventos publicados
	// tenham um destino já na inicialização.
	if err := ch.QueueBind(cfg.Queue, routingKey, cfg.Exchange, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("bind queue: %w", err)
	}

	return &PaymentPublisher{
		conn:     conn,
		channel:  ch,
		exchange: cfg.Exchange,
	}, nil
}

// PublishCreated serializa o pagamento e publica o evento "payment.created".
func (p *PaymentPublisher) PublishCreated(ctx context.Context, pay *payment.Payment) error {
	// Converte a entidade de domínio para o formato de transporte.
	event := paymentCreatedEvent{
		ID:        pay.ID,
		Amount:    pay.Money.Amount,
		Currency:  pay.Money.Currency,
		Status:    string(pay.Status),
		CreatedAt: pay.CreatedAt,
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// Publica na exchange com a routing key. Parâmetros: mandatory=false, immediate=false.
	// DeliveryMode=Persistent grava a mensagem em disco, sobrevivendo a restart do broker.
	err = p.channel.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("publish event: %w", err)
	}

	return nil
}

// Close encerra channel e conexão. Deve ser chamado no shutdown (defer no main).
func (p *PaymentPublisher) Close() error {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
