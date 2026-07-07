package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"payment_service/internal/application/idempotency"
	"payment_service/internal/infrastructure/config"
)

func setupTestRedis(t *testing.T) (*goredis.Client, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return client, cleanup
}

func TestNewClient(t *testing.T) {
	client := NewClient(config.RedisConfig{Host: "localhost", Port: "6379"})
	if client == nil {
		t.Fatal("expected client")
	}
	client.Close()
}

func TestIdempotencyRepository(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	repo := NewIdempotencyRepository(client, time.Minute, time.Hour).(*IdempotencyRepository)

	t.Run("lock and unlock", func(t *testing.T) {
		acquired, err := repo.Lock(ctx, "key-1")
		if err != nil || !acquired {
			t.Fatalf("expected lock acquired, got %v err=%v", acquired, err)
		}

		acquiredAgain, err := repo.Lock(ctx, "key-1")
		if err != nil || acquiredAgain {
			t.Fatalf("expected lock denied, got %v err=%v", acquiredAgain, err)
		}

		if err := repo.Unlock(ctx, "key-1"); err != nil {
			t.Fatalf("unlock failed: %v", err)
		}
	})

	t.Run("save and find", func(t *testing.T) {
		response := idempotency.CachedResponse{
			StatusCode:  201,
			Body:        json.RawMessage(`{"id":"1"}`),
			RequestHash: "hash",
		}

		if err := repo.Save(ctx, "key-2", response); err != nil {
			t.Fatalf("save failed: %v", err)
		}

		found, ok, err := repo.Find(ctx, "key-2")
		if err != nil || !ok {
			t.Fatalf("expected found, ok=%v err=%v", ok, err)
		}
		if found.StatusCode != 201 || found.RequestHash != "hash" {
			t.Fatalf("unexpected response: %+v", found)
		}
	})

	t.Run("find missing key", func(t *testing.T) {
		_, ok, err := repo.Find(ctx, "missing")
		if err != nil || ok {
			t.Fatalf("expected not found, ok=%v err=%v", ok, err)
		}
	})
}

func TestKeyHelpers(t *testing.T) {
	if lockKey("abc") != "idempotency:lock:abc" {
		t.Fatal("unexpected lock key")
	}
	if dataKey("abc") != "idempotency:data:abc" {
		t.Fatal("unexpected data key")
	}
}
