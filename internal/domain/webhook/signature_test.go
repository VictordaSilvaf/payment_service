package webhook

import (
	"strings"
	"testing"
)

func TestSign(t *testing.T) {
	body := []byte(`{"id":"abc"}`)

	sig := Sign("secret", body)
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("expected sha256= prefix, got %s", sig)
	}

	// Determinístico para o mesmo segredo/corpo.
	if Sign("secret", body) != sig {
		t.Fatal("expected deterministic signature")
	}

	// Muda com o segredo.
	if Sign("other", body) == sig {
		t.Fatal("expected different signature for different secret")
	}

	// Muda com o corpo.
	if Sign("secret", []byte(`{"id":"xyz"}`)) == sig {
		t.Fatal("expected different signature for different body")
	}
}
