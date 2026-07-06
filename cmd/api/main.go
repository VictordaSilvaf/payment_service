package main

import (
	"context"
	"log"

	"payment_service/internal/application/usecase"
	"payment_service/internal/database/migrate"
	"payment_service/internal/infrastructure/config"
	"payment_service/internal/infrastructure/http"
	"payment_service/internal/infrastructure/http/handler"
	"payment_service/internal/infrastructure/messaging/rabbitmq"
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

	publisher, err := rabbitmq.NewPaymentPublisher(cfg.RabbitMQ)
	if err != nil {
		log.Fatal(err)
	}
	defer publisher.Close()

	repo := postgres.NewPaymentRepository(pool)

	createPayment := usecase.NewCreatePayment(repo, publisher)
	getPayment := usecase.NewGetPayment(repo)
	listPayment := usecase.NewListPayment(repo)

	router := http.NewRouter(http.RouterConfig{
		HealthHandler:  handler.NewHealthHandler(),
		PaymentHandler: handler.NewPaymentHandler(createPayment, getPayment, listPayment),
	})

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
