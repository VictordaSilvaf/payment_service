package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"payment_service/internal/application/usecase"
	"payment_service/internal/infrastructure/config"
)

// prefetchCount limita quantas mensagens o RabbitMQ entrega antes de receber o Ack.
// Evita que um único consumer puxe milhares de mensagens de uma vez e sobrecarregue.
const prefetchCount = 10

// PaymentConsumer escuta a fila do RabbitMQ e delega o processamento ao use case.
// É um adapter de entrada (driving): traduz mensagens AMQP em chamadas de aplicação.
type PaymentConsumer struct {
	conn           *amqp.Connection
	channel        *amqp.Channel
	queue          string
	processPayment *usecase.ProcessPayment
}

// NewPaymentConsumer abre a conexão, garante que exchange/fila/binding existam
// e configura o QoS. Recebe o use case que será executado a cada mensagem.
func NewPaymentConsumer(
	cfg config.RabbitMQConfig,
	processPayment *usecase.ProcessPayment,
) (*PaymentConsumer, error) {
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
	if err := ch.ExchangeDeclare(cfg.Exchange, exchangeType, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	// Declara a fila (idempotente). Parâmetros:
	// durable=true, autoDelete=false, exclusive=false, noWait=false.
	// Declarar aqui também garante que o consumer funcione mesmo se subir antes do publisher.
	if _, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	// Liga a fila à exchange pela routing key: só mensagens "payment.created" chegam aqui.
	if err := ch.QueueBind(cfg.Queue, routingKey, cfg.Exchange, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("bind queue: %w", err)
	}

	// QoS: no máximo `prefetchCount` mensagens não confirmadas por vez.
	// Parâmetros: prefetchSize=0 (sem limite de bytes), global=false (por consumer).
	if err := ch.Qos(prefetchCount, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}

	return &PaymentConsumer{
		conn:           conn,
		channel:        ch,
		queue:          cfg.Queue,
		processPayment: processPayment,
	}, nil
}

// Start começa a consumir e bloqueia até o contexto ser cancelado (shutdown gracioso)
// ou o channel do RabbitMQ cair.
func (c *PaymentConsumer) Start(ctx context.Context) error {
	// Consume retorna um canal de entregas. autoAck=false → confirmamos manualmente,
	// para não perder mensagem caso o processamento falhe.
	// Demais parâmetros: consumerTag="", exclusive=false, noLocal=false, noWait=false.
	deliveries, err := c.channel.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consuming: %w", err)
	}

	for {
		select {
		// Encerra o loop quando recebe SIGINT/SIGTERM (via contexto do main).
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-deliveries:
			// Canal fechado pelo broker (conexão caiu, por exemplo).
			if !ok {
				return fmt.Errorf("rabbitmq channel closed")
			}
			c.handle(ctx, msg)
		}
	}
}

// handle processa uma única mensagem e decide seu destino via Ack/Nack.
func (c *PaymentConsumer) handle(ctx context.Context, msg amqp.Delivery) {
	var event paymentCreatedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		// Mensagem malformada: Nack com requeue=false descarta (evita loop infinito).
		log.Printf("invalid message, discarding: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	// Traduz o evento (formato de transporte) para o input do use case (formato de aplicação).
	input := usecase.ProcessPaymentInput{
		PaymentID: event.ID,
		Amount:    event.Amount,
		Currency:  event.Currency,
	}

	if _, err := c.processPayment.Execute(ctx, input); err != nil {
		// Falha no processamento: Nack com requeue=true devolve à fila para nova tentativa.
		log.Printf("failed to process payment %s, requeueing: %v", event.ID, err)
		_ = msg.Nack(false, true)
		return
	}

	// Sucesso: Ack confirma e remove a mensagem da fila. multiple=false (só esta mensagem).
	log.Printf("payment %s processed", event.ID)
	_ = msg.Ack(false)
}

// Close encerra channel e conexão. Deve ser chamado no shutdown (defer no main).
func (c *PaymentConsumer) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
