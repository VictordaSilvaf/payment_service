package factory

import (
	"math/rand"
	"time"

	"github.com/google/uuid"

	"payment_service/internal/domain/payment"
)

var (
	currencies = []string{"BRL", "USD", "EUR"}
	statuses   = []payment.Status{
		payment.StatusPending,
		payment.StatusCompleted,
		payment.StatusFailed,
	}
)

type PaymentFactory struct {
	rng      *rand.Rand
	amount   *int64
	currency *string
	status   *payment.Status
	created  *time.Time
}

func NewPaymentFactory() *PaymentFactory {
	return &PaymentFactory{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (f *PaymentFactory) WithAmount(amount int64) *PaymentFactory {
	f.amount = &amount
	return f
}

func (f *PaymentFactory) WithCurrency(currency string) *PaymentFactory {
	f.currency = &currency
	return f
}

func (f *PaymentFactory) WithStatus(status payment.Status) *PaymentFactory {
	f.status = &status
	return f
}

func (f *PaymentFactory) WithCreatedAt(createdAt time.Time) *PaymentFactory {
	f.created = &createdAt
	return f
}

func (f *PaymentFactory) Make() *payment.Payment {
	amount := f.resolveAmount()
	currency := f.resolveCurrency()
	status := f.resolveStatus()
	createdAt := f.resolveCreatedAt()

	return &payment.Payment{
		ID:        uuid.New().String(),
		Money:     payment.Money{Amount: amount, Currency: currency},
		Status:    status,
		CreatedAt: createdAt,
	}
}

func (f *PaymentFactory) MakeMany(count int) []*payment.Payment {
	payments := make([]*payment.Payment, count)
	for i := range count {
		payments[i] = f.Make()
	}
	return payments
}

func (f *PaymentFactory) resolveAmount() int64 {
	if f.amount != nil {
		return *f.amount
	}
	return int64(f.rng.Intn(990_000)+1_000) // 10.00 to 9900.00 in cents
}

func (f *PaymentFactory) resolveCurrency() string {
	if f.currency != nil {
		return *f.currency
	}
	return currencies[f.rng.Intn(len(currencies))]
}

func (f *PaymentFactory) resolveStatus() payment.Status {
	if f.status != nil {
		return *f.status
	}
	return statuses[f.rng.Intn(len(statuses))]
}

func (f *PaymentFactory) resolveCreatedAt() time.Time {
	if f.created != nil {
		return f.created.UTC()
	}
	daysAgo := f.rng.Intn(90)
	return time.Now().UTC().AddDate(0, 0, -daysAgo)
}
