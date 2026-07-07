package main

import (
	"context"
	"log"

	"payment_service/internal/application/idempotency"
	"payment_service/internal/application/usecase"
	appwebhook "payment_service/internal/application/webhook"
	"payment_service/internal/database/migrate"
	"payment_service/internal/infrastructure/cache/redis"
	"payment_service/internal/infrastructure/config"
	"payment_service/internal/infrastructure/http"
	"payment_service/internal/infrastructure/http/handler"
	"payment_service/internal/infrastructure/persistence/postgres"
	"payment_service/internal/infrastructure/psp"
)

func main() {
	cfg := config.Load()

	if err := migrate.Up(cfg.Postgres.DSN()); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	redisClient := redis.NewClient(cfg.Redis)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal(err)
	}

	repo := postgres.NewPaymentRepository(pool)
	outboxRepo := postgres.NewOutboxRepository(pool)
	txManager := postgres.NewTxManager(pool)

	idempotencyRepo := redis.NewIdempotencyRepository(
		redisClient,
		cfg.IdempotencyLock,
		cfg.IdempotencyTTL,
	)
	idempotencyService := idempotency.NewService(idempotencyRepo)

	// Captura e estorno falam com o PSP de forma síncrona (ação do lojista via API),
	// diferente da autorização, que é assíncrona (consumer).
	gateway := psp.NewMockGateway(cfg.PSP.MockLatency)

	createPayment := usecase.NewCreatePayment(repo, outboxRepo, txManager)
	getPayment := usecase.NewGetPayment(repo)
	listPayment := usecase.NewListPayment(repo)
	capturePayment := usecase.NewCapturePayment(repo, gateway, outboxRepo, txManager)
	refundPayment := usecase.NewRefundPayment(repo, gateway, outboxRepo, txManager)

	webhookSubscriptions := postgres.NewWebhookSubscriptionRepository(pool)
	createSubscription := appwebhook.NewCreateSubscription(webhookSubscriptions)
	listSubscriptions := appwebhook.NewListSubscriptions(webhookSubscriptions)

	router := http.NewRouter(http.RouterConfig{
		HealthHandler: handler.NewHealthHandler(),
		PaymentHandler: handler.NewPaymentHandler(
			createPayment,
			getPayment,
			listPayment,
			capturePayment,
			refundPayment,
			idempotencyService,
		),
		WebhookHandler: handler.NewWebhookHandler(
			createSubscription,
			listSubscriptions,
		),
	})

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
