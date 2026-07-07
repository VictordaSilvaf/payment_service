package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	appnotification "payment_service/internal/application/notification"
	domainnotification "payment_service/internal/domain/notification"
	"payment_service/internal/infrastructure/config"
	"payment_service/internal/infrastructure/messaging/rabbitmq"
	infranotification "payment_service/internal/infrastructure/notification"
	"payment_service/internal/infrastructure/persistence/postgres"
)

func main() {
	cfg := config.Load()

	db, err := postgres.NewPool(context.Background(), cfg.Postgres)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	notifications := postgres.NewNotificationRepository(db)
	notifier := infranotification.NewLogNotifier()

	notify := appnotification.NewNotifyPayment(
		notifier,
		notifications,
		domainnotification.Channel(cfg.Notification.Channel),
	)

	// Consome os eventos de pagamento publicados pelo relay e notifica o usuário
	// final de cada resultado (concluído, recusado ou estornado). Falhas de envio
	// são retentadas e, se persistirem, encaminhadas à DLQ pelo Subscriber.
	subscriber, err := rabbitmq.NewSubscriber(
		cfg.RabbitMQ,
		cfg.Notification.Queue,
		[]string{"payment.completed", "payment.failed", "payment.refunded"},
		func(ctx context.Context, routingKey string, body []byte) error {
			return notify.Execute(ctx, routingKey, body)
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

	log.Println("🚀 Notification Service iniciado")

	if err := subscriber.Start(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
