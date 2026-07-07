package webhook

import (
	"errors"
	"testing"
)

func TestNewSubscription(t *testing.T) {
	t.Run("valid with explicit secret", func(t *testing.T) {
		s, err := NewSubscription("https://merchant.test/hook", "s3cr3t", "payment.completed")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.ID == "" {
			t.Fatal("expected generated id")
		}
		if s.Secret != "s3cr3t" {
			t.Fatalf("unexpected secret: %s", s.Secret)
		}
		if !s.Active {
			t.Fatal("expected subscription to be active")
		}
		if s.CreatedAt.IsZero() {
			t.Fatal("expected created_at to be set")
		}
	})

	t.Run("generates secret when empty", func(t *testing.T) {
		s, err := NewSubscription("https://merchant.test/hook", "", "payment.completed")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Secret == "" {
			t.Fatal("expected generated secret")
		}
	})

	t.Run("invalid url", func(t *testing.T) {
		for _, raw := range []string{"", "ftp://x", "not-a-url", "://missing"} {
			if _, err := NewSubscription(raw, "s", "payment.completed"); !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("expected ErrInvalidURL for %q, got %v", raw, err)
			}
		}
	})

	t.Run("invalid event type", func(t *testing.T) {
		if _, err := NewSubscription("https://merchant.test/hook", "s", "  "); !errors.Is(err, ErrInvalidEventType) {
			t.Fatalf("expected ErrInvalidEventType, got %v", err)
		}
	})
}
