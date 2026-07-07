package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"payment_service/internal/application/usecase"
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

	paymentRepository := postgres.NewPaymentRepository(db)
	outboxRepository := postgres.NewOutboxRepository(db)
	txManager := postgres.NewTxManager(db)
	processPayment := usecase.NewProcessPayment(paymentRepository, outboxRepository, txManager)

	consumer, err := rabbitmq.NewPaymentConsumer(
		cfg.RabbitMQ,
		processPayment,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Println("🚀 Payment Consumer iniciado")

	if err := consumer.Start(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
