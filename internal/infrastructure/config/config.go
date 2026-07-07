package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	Postgres        PostgresConfig
	Redis           RedisConfig
	RabbitMQ        RabbitMQConfig
	IdempotencyTTL  time.Duration
	IdempotencyLock time.Duration
	Outbox          OutboxConfig
	Webhook         WebhookConfig
}

type OutboxConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

type WebhookConfig struct {
	Queue       string        // fila do webhook service
	HTTPTimeout time.Duration // timeout do POST ao endpoint do lojista
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		p.User, p.Password, p.Host, p.Port, p.DBName,
	)
}

type RedisConfig struct {
	Host string
	Port string
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

type RabbitMQConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
	Exchange string
	Queue    string
}

func (r RabbitMQConfig) URL() string {
	base := fmt.Sprintf("amqp://%s:%s@%s:%s", r.User, r.Password, r.Host, r.Port)
	if r.VHost == "" || r.VHost == "/" {
		return base + "/"
	}
	return base + "/" + r.VHost
}

func Load() Config {
	_ = godotenv.Load()

	idempotencyTTL := durationOrDefault("IDEMPOTENCY_TTL", 24*time.Hour)
	idempotencyLock := durationOrDefault("IDEMPOTENCY_LOCK_TTL", 30*time.Second)

	return Config{
		Port:            envOrDefault("PORT", "8080"),
		IdempotencyTTL:  idempotencyTTL,
		IdempotencyLock: idempotencyLock,
		Outbox: OutboxConfig{
			PollInterval: durationOrDefault("OUTBOX_POLL_INTERVAL", time.Second),
			BatchSize:    intOrDefault("OUTBOX_BATCH_SIZE", 100),
		},
		Webhook: WebhookConfig{
			Queue:       envOrDefault("WEBHOOK_QUEUE", "webhook.payment"),
			HTTPTimeout: durationOrDefault("WEBHOOK_HTTP_TIMEOUT", 5*time.Second),
		},
		Postgres: PostgresConfig{
			Host:     envOrDefault("POSTGRES_HOST", "localhost"),
			Port:     envOrDefault("POSTGRES_PORT", "5432"),
			User:     envOrDefault("POSTGRES_USER", "payment"),
			Password: envOrDefault("POSTGRES_PASSWORD", "payment"),
			DBName:   envOrDefault("POSTGRES_DB", "payment_db"),
		},
		Redis: RedisConfig{
			Host: envOrDefault("REDIS_HOST", "localhost"),
			Port: envOrDefault("REDIS_PORT", "6379"),
		},
		RabbitMQ: RabbitMQConfig{
			Host:     envOrDefault("RABBITMQ_HOST", "localhost"),
			Port:     envOrDefault("RABBITMQ_PORT", "5672"),
			User:     envOrDefault("RABBITMQ_USER", "payment"),
			Password: envOrDefault("RABBITMQ_PASSWORD", "payment"),
			VHost:    envOrDefault("RABBITMQ_VHOST", "/"),
			Exchange: envOrDefault("RABBITMQ_EXCHANGE", "payment.events"),
			Queue:    envOrDefault("RABBITMQ_QUEUE", "payment.created"),
		},
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}

	return parsed
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}
