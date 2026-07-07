.PHONY: migrate-up migrate-down migrate-version seed seed-fresh run test test-coverage

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-version:
	go run ./cmd/migrate version

seed:
	go run ./cmd/seed

seed-fresh:
	go run ./cmd/seed -fresh

run:
	go run ./cmd/api

TEST_PACKAGES := ./internal/application/... ./internal/domain/... ./internal/infrastructure/cache/... ./internal/infrastructure/config/... ./internal/infrastructure/http/... ./internal/infrastructure/persistence/memory/... ./internal/database/factory/...

test:
	go test $(TEST_PACKAGES) -count=1

test-coverage:
	go test $(TEST_PACKAGES) -count=1 -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out | tail -1
	@total=$$(go tool cover -func=coverage.out | awk '/total:/ {print $$3}' | tr -d '%'); \
	if [ "$${total%%.*}" -lt 80 ]; then \
		echo "coverage below 80% ($$total%)"; \
		exit 1; \
	fi
