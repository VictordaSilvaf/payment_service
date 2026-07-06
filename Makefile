.PHONY: migrate-up migrate-down migrate-version seed seed-fresh run

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-version:
	go run ./cmd/migrate version

seed:
	go run ./cmd/seed -count=25

seed-fresh:
	go run ./cmd/seed -count=25 -fresh

run:
	go run ./cmd/api
