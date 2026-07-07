package rabbitmq

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// dlxSuffix/dlqSuffix nomeiam a Dead Letter Exchange e a Dead Letter Queue a
// partir da exchange e da fila principais.
const (
	dlxSuffix = ".dlx"
	dlqSuffix = ".dlq"
)

func dlxName(exchange string) string { return exchange + dlxSuffix }
func dlqName(queue string) string    { return queue + dlqSuffix }

// declareDLQ cria a Dead Letter Exchange (direct) e a Dead Letter Queue, ligando
// a DLQ à DLX pela routing key = nome da fila principal. Devolve os argumentos
// que devem ser aplicados à fila principal para que mensagens rejeitadas
// (Nack com requeue=false) sejam roteadas para a DLQ.
//
// Observação: os argumentos de uma fila são imutáveis. Se a fila principal já
// existir sem esses argumentos, o QueueDeclare falhará (PRECONDITION_FAILED) e
// será preciso removê-la antes (em dev, `docker compose down -v`).
func declareDLQ(ch *amqp.Channel, exchange, queue string) (amqp.Table, error) {
	dlx := dlxName(exchange)
	dlq := dlqName(queue)

	// DLX própria (direct): isola o roteamento de mensagens mortas.
	if err := ch.ExchangeDeclare(dlx, "direct", true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare dlx: %w", err)
	}

	// DLQ durável, sem consumidores automáticos (é um "estacionamento").
	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare dlq: %w", err)
	}

	// Liga a DLQ à DLX pela routing key = nome da fila principal.
	if err := ch.QueueBind(dlq, queue, dlx, false, nil); err != nil {
		return nil, fmt.Errorf("bind dlq: %w", err)
	}

	// Argumentos da fila principal: para onde e com qual chave despachar os mortos.
	return amqp.Table{
		"x-dead-letter-exchange":    dlx,
		"x-dead-letter-routing-key": queue,
	}, nil
}

// processWithRetry executa fn até maxRetries+1 vezes, aguardando `delay` entre as
// tentativas e respeitando o cancelamento do contexto. Retorna nil no primeiro
// sucesso ou o último erro após esgotar as tentativas — sinal para o chamador
// enviar a mensagem à DLQ.
func processWithRetry(ctx context.Context, maxRetries int, delay time.Duration, fn func() error) error {
	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if attempt == maxRetries {
			break
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return err
		}
	}
	return err
}
