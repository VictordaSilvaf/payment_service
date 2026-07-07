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

	policy := appwebhook.BackoffPolicy{
		MaxAttempts: cfg.Webhook.Retry.MaxAttempts,
		BaseDelay:   cfg.Webhook.Retry.BaseDelay,
	}

	dispatch := appwebhook.NewDispatchWebhook(subscriptions, deliveries, sender, policy)
	retrier := appwebhook.NewRetryDeliveries(
		deliveries,
		sender,
		policy,
		cfg.Webhook.Retry.PollInterval,
		cfg.Webhook.Retry.BatchSize,
	)

	// Consome os eventos de pagamento publicados pelo relay e dispara os webhooks
	// para as assinaturas ativas de cada tipo (concluído, recusado ou estornado).
	subscriber, err := rabbitmq.NewSubscriber(
		cfg.RabbitMQ,
		cfg.Webhook.Queue,
		[]string{"payment.completed", "payment.failed", "payment.refunded"},
		func(ctx context.Context, msg rabbitmq.Message) error {
			return dispatch.Execute(ctx, msg.RoutingKey, msg.Body)
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

	// Poller de retry roda em paralelo ao consumer, reenviando entregas que
	// falharam (backoff) até o limite de tentativas.
	go func() {
		if err := retrier.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("webhook retry loop stopped: %v", err)
		}
	}()

	log.Println("🚀 Webhook Service iniciado (entrega + retry)")

	if err := subscriber.Start(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
