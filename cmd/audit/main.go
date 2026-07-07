package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	appaudit "payment_service/internal/application/audit"
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

	audits := postgres.NewAuditRepository(db)
	record := appaudit.NewRecordAudit(audits)

	// Assina TODOS os eventos de pagamento (payment.*) para registrar uma trilha
	// completa e imutável. Cada evento é gravado uma única vez (dedup pelo id da
	// mensagem); falhas são retentadas e, se persistirem, vão para a DLQ.
	subscriber, err := rabbitmq.NewSubscriber(
		cfg.RabbitMQ,
		cfg.Audit.Queue,
		[]string{"payment.*"},
		func(ctx context.Context, msg rabbitmq.Message) error {
			return record.Execute(ctx, msg.ID, msg.RoutingKey, msg.Body)
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	defer subscriber.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Println("🚀 Audit Service iniciado")

	if err := subscriber.Start(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
