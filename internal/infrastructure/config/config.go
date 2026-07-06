package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port     string
	Postgres PostgresConfig
	Redis    RedisConfig
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

func Load() Config {
	return Config{
		Port: envOrDefault("PORT", "8080"),
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
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
