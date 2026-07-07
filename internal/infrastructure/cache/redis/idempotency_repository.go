package redis

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"payment_service/internal/application/idempotency"
	"payment_service/internal/infrastructure/config"
)

func NewClient(cfg config.RedisConfig) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr: cfg.Addr(),
	})
}

type IdempotencyRepository struct {
	client  *goredis.Client
	lockTTL time.Duration
	dataTTL time.Duration
}

func NewIdempotencyRepository(
	client *goredis.Client,
	lockTTL time.Duration,
	dataTTL time.Duration,
) idempotency.Repository {
	return &IdempotencyRepository{
		client:  client,
		lockTTL: lockTTL,
		dataTTL: dataTTL,
	}
}

func (r *IdempotencyRepository) Lock(ctx context.Context, key string) (bool, error) {
	return r.client.SetNX(ctx, lockKey(key), "processing", r.lockTTL).Result()
}

func (r *IdempotencyRepository) Unlock(ctx context.Context, key string) error {
	return r.client.Del(ctx, lockKey(key)).Err()
}

func (r *IdempotencyRepository) Save(ctx context.Context, key string, response idempotency.CachedResponse) error {
	value, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, dataKey(key), value, r.dataTTL).Err()
}

func (r *IdempotencyRepository) Find(ctx context.Context, key string) (idempotency.CachedResponse, bool, error) {
	value, err := r.client.Get(ctx, dataKey(key)).Bytes()
	if err == goredis.Nil {
		return idempotency.CachedResponse{}, false, nil
	}
	if err != nil {
		return idempotency.CachedResponse{}, false, err
	}

	var response idempotency.CachedResponse
	if err := json.Unmarshal(value, &response); err != nil {
		return idempotency.CachedResponse{}, false, err
	}

	return response, true, nil
}

func lockKey(key string) string {
	return "idempotency:lock:" + key
}

func dataKey(key string) string {
	return "idempotency:data:" + key
}
