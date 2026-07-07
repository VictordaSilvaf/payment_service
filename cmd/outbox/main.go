package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	appoutbox "payment_service/internal/application/outbox"
	"payment_service/internal/infrastructure/config"
	"payment_service/internal/infrastructure/messaging/rabbitmq"
	"payment_service/internal/infrastructure/persistence/postgres"
)

func main() {
	cfg := config.Load()

	db, err := postgres.NewPool(context.Background(), cfg.Postgres)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Reusa o publisher do RabbitMQ (declara exchange/fila/binding na inicialização).
	publisher, err := rabbitmq.NewPaymentPublisher(cfg.RabbitMQ)
	if err != nil {
		log.Fatal(err)
	}
	defer publisher.Close()

	outboxRepo := postgres.NewOutboxRepository(db)
	relay := appoutbox.NewRelay(
		outboxRepo,
		publisher,
		cfg.Outbox.PollInterval,
		cfg.Outbox.BatchSize,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Println("🚀 Outbox Relay iniciado")

	if err := relay.Run(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
