package main

import (
	"context"
	"log"

	"payment_service/internal/application/idempotency"
	"payment_service/internal/application/usecase"
	"payment_service/internal/database/migrate"
	"payment_service/internal/infrastructure/cache/redis"
	"payment_service/internal/infrastructure/config"
	"payment_service/internal/infrastructure/http"
	"payment_service/internal/infrastructure/http/handler"
	"payment_service/internal/infrastructure/persistence/postgres"
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

	createPayment := usecase.NewCreatePayment(repo, outboxRepo, txManager)
	getPayment := usecase.NewGetPayment(repo)
	listPayment := usecase.NewListPayment(repo)

	router := http.NewRouter(http.RouterConfig{
		HealthHandler: handler.NewHealthHandler(),
		PaymentHandler: handler.NewPaymentHandler(
			createPayment,
			getPayment,
			listPayment,
			idempotencyService,
		),
	})

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
