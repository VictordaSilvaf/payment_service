package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"

	"payment_service/internal/database/seeder"
	"payment_service/internal/infrastructure/config"
	"payment_service/internal/infrastructure/persistence/postgres"
)

func main() {
	cfg := config.Load()

	count := flag.Int("count", seedCountDefault(), "number of payments to seed")
	fresh := flag.Bool("fresh", false, "truncate payments table before seeding")
	flag.Parse()

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

func seedCountDefault() int {
	if value := os.Getenv("SEED_COUNT"); value != "" {
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			return n
		}
	}
	return 25
}
