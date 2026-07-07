package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type mockRepository struct {
	findResp     CachedResponse
	findFound    bool
	findErr      error
	lockAcquired bool
	lockErr      error
	saveErr      error
	unlockErr    error
	saved        map[string]CachedResponse
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		lockAcquired: true,
		saved:        make(map[string]CachedResponse),
	}
}

func (m *mockRepository) Lock(_ context.Context, _ string) (bool, error) {
	return m.lockAcquired, m.lockErr
}

func (m *mockRepository) Unlock(_ context.Context, _ string) error {
	return m.unlockErr
}

func (m *mockRepository) Save(_ context.Context, key string, response CachedResponse) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved[key] = response
	return nil
}

func (m *mockRepository) Find(_ context.Context, _ string) (CachedResponse, bool, error) {
	if m.findErr != nil {
		return CachedResponse{}, false, m.findErr
	}
	return m.findResp, m.findFound, nil
}

func TestServiceExecute(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid key", func(t *testing.T) {
		svc := NewService(newMockRepository())
		_, err := svc.Execute(ctx, "  ", "hash", nil)
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("expected ErrInvalidKey, got %v", err)
		}
	})

	t.Run("cache hit", func(t *testing.T) {
		repo := newMockRepository()
		repo.findFound = true
		repo.findResp = CachedResponse{StatusCode: 201, Body: json.RawMessage(`{"id":"1"}`), RequestHash: "abc"}

		svc := NewService(repo)
		resp, err := svc.Execute(ctx, "key-1", "abc", func(context.Context) (CachedResponse, error) {
			t.Fatal("fn should not be called on cache hit")
			return CachedResponse{}, nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 201 {
			t.Fatalf("unexpected status: %d", resp.StatusCode)
		}
	})

	t.Run("cache hit with conflicting hash", func(t *testing.T) {
		repo := newMockRepository()
		repo.findFound = true
		repo.findResp = CachedResponse{StatusCode: 201, RequestHash: "other"}

		svc := NewService(repo)
		_, err := svc.Execute(ctx, "key-1", "abc", nil)
		if !errors.Is(err, ErrKeyAlreadyExists) {
			t.Fatalf("expected ErrKeyAlreadyExists, got %v", err)
		}
	})

	t.Run("lock not acquired and no cache", func(t *testing.T) {
		repo := newMockRepository()
		repo.lockAcquired = false

		svc := NewService(repo)
		_, err := svc.Execute(ctx, "key-1", "abc", nil)
		if !errors.Is(err, ErrAlreadyProcessing) {
			t.Fatalf("expected ErrAlreadyProcessing, got %v", err)
		}
	})

	t.Run("executes and saves success response", func(t *testing.T) {
		repo := newMockRepository()
		svc := NewService(repo)

		resp, err := svc.Execute(ctx, "key-1", "abc", func(context.Context) (CachedResponse, error) {
			return CachedResponse{StatusCode: 201, Body: json.RawMessage(`{"ok":true}`)}, nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 201 || resp.RequestHash != "abc" {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if _, ok := repo.saved["key-1"]; !ok {
			t.Fatal("expected response to be saved")
		}
	})

	t.Run("does not save non success response", func(t *testing.T) {
		repo := newMockRepository()
		svc := NewService(repo)

		resp, err := svc.Execute(ctx, "key-2", "abc", func(context.Context) (CachedResponse, error) {
			return CachedResponse{StatusCode: 400, Body: json.RawMessage(`{"error":"bad"}`)}, nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 400 {
			t.Fatalf("unexpected status: %d", resp.StatusCode)
		}
		if len(repo.saved) != 0 {
			t.Fatal("expected response not to be saved")
		}
	})

	t.Run("fn error", func(t *testing.T) {
		repo := newMockRepository()
		svc := NewService(repo)
		fnErr := errors.New("boom")

		_, err := svc.Execute(ctx, "key-3", "abc", func(context.Context) (CachedResponse, error) {
			return CachedResponse{}, fnErr
		})
		if !errors.Is(err, fnErr) {
			t.Fatalf("expected fn error, got %v", err)
		}
	})

	t.Run("find error", func(t *testing.T) {
		repo := newMockRepository()
		repo.findErr = errors.New("redis down")
		svc := NewService(repo)

		_, err := svc.Execute(ctx, "key-4", "abc", nil)
		if !errors.Is(err, repo.findErr) {
			t.Fatalf("expected find error, got %v", err)
		}
	})

	t.Run("save error", func(t *testing.T) {
		repo := newMockRepository()
		repo.saveErr = errors.New("save failed")
		svc := NewService(repo)

		_, err := svc.Execute(ctx, "key-5", "abc", func(context.Context) (CachedResponse, error) {
			return CachedResponse{StatusCode: 201, Body: json.RawMessage(`{}`)}, nil
		})
		if !errors.Is(err, repo.saveErr) {
			t.Fatalf("expected save error, got %v", err)
		}
	})
}

func TestValidateKey(t *testing.T) {
	longKey := make([]byte, 256)
	for i := range longKey {
		longKey[i] = 'a'
	}

	if err := validateKey("valid"); err != nil {
		t.Fatalf("expected valid key, got %v", err)
	}
	if err := validateKey(""); err != ErrInvalidKey {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
	if err := validateKey(string(longKey)); err != ErrInvalidKey {
		t.Fatalf("expected ErrInvalidKey for long key, got %v", err)
	}
}
