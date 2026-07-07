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
	PSP             PSPConfig
	Notification    NotificationConfig
	Audit           AuditConfig
}

type NotificationConfig struct {
	Queue   string // fila do notification service
	Channel string // canal padrão de envio (email/sms/push)
}

type AuditConfig struct {
	Queue string // fila do audit service
}

type PSPConfig struct {
	MockLatency time.Duration // latência simulada da chamada ao PSP mock
}

type OutboxConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

type WebhookConfig struct {
	Queue       string        // fila do webhook service
	HTTPTimeout time.Duration // timeout do POST ao endpoint do lojista
	Retry       RetryConfig
}

type RetryConfig struct {
	MaxAttempts  int           // máximo de tentativas antes de esgotar (exhausted)
	BaseDelay    time.Duration // atraso base do backoff exponencial
	PollInterval time.Duration // intervalo de varredura do poller de retry
	BatchSize    int           // máx. de entregas retentadas por ciclo
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
	Host       string
	Port       string
	User       string
	Password   string
	VHost      string
	Exchange   string
	Queue      string
	MaxRetries int           // tentativas de processamento antes de mandar à DLQ
	RetryDelay time.Duration // espera entre as tentativas de processamento
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
			Retry: RetryConfig{
				MaxAttempts:  intOrDefault("WEBHOOK_RETRY_MAX_ATTEMPTS", 5),
				BaseDelay:    durationOrDefault("WEBHOOK_RETRY_BASE_DELAY", 30*time.Second),
				PollInterval: durationOrDefault("WEBHOOK_RETRY_POLL_INTERVAL", 10*time.Second),
				BatchSize:    intOrDefault("WEBHOOK_RETRY_BATCH_SIZE", 100),
			},
		},
		PSP: PSPConfig{
			MockLatency: durationOrDefault("PSP_MOCK_LATENCY", 0),
		},
		Notification: NotificationConfig{
			Queue:   envOrDefault("NOTIFICATION_QUEUE", "notification.payment"),
			Channel: envOrDefault("NOTIFICATION_CHANNEL", "email"),
		},
		Audit: AuditConfig{
			Queue: envOrDefault("AUDIT_QUEUE", "audit.payment"),
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
			Host:       envOrDefault("RABBITMQ_HOST", "localhost"),
			Port:       envOrDefault("RABBITMQ_PORT", "5672"),
			User:       envOrDefault("RABBITMQ_USER", "payment"),
			Password:   envOrDefault("RABBITMQ_PASSWORD", "payment"),
			VHost:      envOrDefault("RABBITMQ_VHOST", "/"),
			Exchange:   envOrDefault("RABBITMQ_EXCHANGE", "payment.events"),
			Queue:      envOrDefault("RABBITMQ_QUEUE", "payment.created"),
			MaxRetries: intOrDefault("RABBITMQ_MAX_RETRIES", 3),
			RetryDelay: durationOrDefault("RABBITMQ_RETRY_DELAY", 2*time.Second),
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
