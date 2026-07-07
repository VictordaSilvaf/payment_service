package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	appwebhook "payment_service/internal/application/webhook"
	"payment_service/internal/infrastructure/config"
	"payment_service/internal/infrastructure/http/webhookclient"
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

	subscriptions := postgres.NewWebhookSubscriptionRepository(db)
	deliveries := postgres.NewWebhookDeliveryRepository(db)
	sender := webhookclient.New(cfg.Webhook.HTTPTimeout)

	dispatch := appwebhook.NewDispatchWebhook(subscriptions, deliveries, sender)

	// Consome os eventos de pagamento publicados pelo relay e dispara os webhooks
	// para as assinaturas ativas de cada tipo (concluído ou recusado pelo PSP).
	subscriber, err := rabbitmq.NewSubscriber(
		cfg.RabbitMQ,
		cfg.Webhook.Queue,
		[]string{"payment.completed", "payment.failed"},
		func(ctx context.Context, routingKey string, body []byte) error {
			return dispatch.Execute(ctx, routingKey, body)
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

	log.Println("🚀 Webhook Service iniciado")

	if err := subscriber.Start(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
