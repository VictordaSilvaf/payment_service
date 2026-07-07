package payment

import "testing"

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	if v == nil || v.validator == nil {
		t.Fatal("expected validator instance")
	}
}
