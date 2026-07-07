package config

import (
	"os"
	"testing"
	"time"
)

func TestPostgresDSN(t *testing.T) {
	dsn := PostgresConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "user",
		Password: "pass",
		DBName:   "db",
	}.DSN()

	expected := "postgres://user:pass@localhost:5432/db?sslmode=disable"
	if dsn != expected {
		t.Fatalf("unexpected dsn: %s", dsn)
	}
}

func TestRedisAddr(t *testing.T) {
	addr := RedisConfig{Host: "redis", Port: "6379"}.Addr()
	if addr != "redis:6379" {
		t.Fatalf("unexpected addr: %s", addr)
	}
}

func TestRabbitMQURL(t *testing.T) {
	t.Run("default vhost", func(t *testing.T) {
		url := RabbitMQConfig{
			Host: "localhost", Port: "5672", User: "u", Password: "p", VHost: "/",
		}.URL()
		if url != "amqp://u:p@localhost:5672/" {
			t.Fatalf("unexpected url: %s", url)
		}
	})

	t.Run("custom vhost", func(t *testing.T) {
		url := RabbitMQConfig{
			Host: "localhost", Port: "5672", User: "u", Password: "p", VHost: "wallet",
		}.URL()
		if url != "amqp://u:p@localhost:5672/wallet" {
			t.Fatalf("unexpected url: %s", url)
		}
	})
}

func TestLoad(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("POSTGRES_USER", "wallet")
	t.Setenv("IDEMPOTENCY_TTL", "2h")
	t.Setenv("IDEMPOTENCY_LOCK_TTL", "invalid")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Fatalf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.Postgres.User != "wallet" {
		t.Fatalf("expected postgres user wallet, got %s", cfg.Postgres.User)
	}
	if cfg.IdempotencyTTL != 2*time.Hour {
		t.Fatalf("expected 2h ttl, got %s", cfg.IdempotencyTTL)
	}
	if cfg.IdempotencyLock != 30*time.Second {
		t.Fatalf("expected fallback lock ttl, got %s", cfg.IdempotencyLock)
	}
}

func TestEnvOrDefault(t *testing.T) {
	key := "TEST_ENV_OR_DEFAULT_KEY"
	os.Unsetenv(key)

	if envOrDefault(key, "fallback") != "fallback" {
		t.Fatal("expected fallback")
	}

	t.Setenv(key, "value")
	if envOrDefault(key, "fallback") != "value" {
		t.Fatal("expected env value")
	}
}

func TestDurationOrDefault(t *testing.T) {
	key := "TEST_DURATION_OR_DEFAULT_KEY"
	os.Unsetenv(key)

	if durationOrDefault(key, time.Minute) != time.Minute {
		t.Fatal("expected fallback duration")
	}

	t.Setenv(key, "10m")
	if durationOrDefault(key, time.Minute) != 10*time.Minute {
		t.Fatal("expected parsed duration")
	}

	t.Setenv(key, "bad")
	if durationOrDefault(key, time.Minute) != time.Minute {
		t.Fatal("expected fallback for invalid duration")
	}
}
