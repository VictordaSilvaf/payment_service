package main

import (
	"context"
	"flag"
	"log"

	"payment_service/internal/database/seeder"
	"payment_service/internal/infrastructure/config"
	"payment_service/internal/infrastructure/persistence/postgres"
)

func main() {
	count := flag.Int("count", 25, "number of payments to seed")
	fresh := flag.Bool("fresh", false, "truncate payments table before seeding")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repo := postgres.NewPaymentRepository(pool)
	registry := seeder.NewRegistry(
		seeder.NewPaymentSeeder(repo, *count, *fresh),
	)

	if err := registry.RunAll(ctx); err != nil {
		log.Fatal(err)
	}

	log.Printf("seeded %d payments successfully", *count)
}
