package payment

import (
	"testing"
	"time"

	domainpayment "payment_service/internal/domain/payment"
	"payment_service/internal/application/dto"
)

func TestToDomain(t *testing.T) {
	p, err := ToDomain(dto.CreatePaymentRequest{Amount: 100, Currency: "BRL"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Money.Amount != 100 {
		t.Fatalf("unexpected amount: %d", p.Money.Amount)
	}
}

func TestToResponseAndResponses(t *testing.T) {
	created := time.Now().UTC()
	p := &domainpayment.Payment{
		ID:        "id-1",
		Money:     domainpayment.Money{Amount: 200, Currency: "USD"},
		Status:    domainpayment.StatusPending,
		CreatedAt: created,
	}

	resp := ToResponse(p)
	if resp.ID != "id-1" || resp.Amount != 200 || resp.Currency != "USD" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	responses := ToResponses([]*domainpayment.Payment{p})
	if len(responses) != 1 || responses[0].ID != "id-1" {
		t.Fatalf("unexpected responses: %+v", responses)
	}
}
