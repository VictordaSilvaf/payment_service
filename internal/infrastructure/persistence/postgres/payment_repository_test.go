package postgres

import "testing"

func TestParsePositiveIntPostgres(t *testing.T) {
	if parsePositiveInt("5", 10) != 5 {
		t.Fatal("expected 5")
	}
	if parsePositiveInt("", 10) != 10 {
		t.Fatal("expected fallback")
	}
}

func TestSanitizeSort(t *testing.T) {
	cases := map[string]string{
		"id":         "id",
		"amount":     "amount",
		"currency":   "currency",
		"status":     "status",
		"invalid":    "created_at",
		"CREATED_AT": "created_at",
	}

	for input, expected := range cases {
		if got := sanitizeSort(input); got != expected {
			t.Fatalf("sanitizeSort(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestSanitizeOrder(t *testing.T) {
	if sanitizeOrder("asc") != "ASC" {
		t.Fatal("expected ASC")
	}
	if sanitizeOrder("ASC") != "ASC" {
		t.Fatal("expected ASC")
	}
	if sanitizeOrder("desc") != "DESC" {
		t.Fatal("expected DESC")
	}
}
