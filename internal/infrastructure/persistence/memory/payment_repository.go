package memory

import (
	"context"
	"strconv"
	"sync"

	"payment_service/internal/domain/payment"
)

type PaymentRepository struct {
	mu       sync.RWMutex
	payments map[string]*payment.Payment
}

func NewPaymentRepository() *PaymentRepository {
	return &PaymentRepository{
		payments: make(map[string]*payment.Payment),
	}
}

func (r *PaymentRepository) FindAll(_ context.Context, page, limit, _, _, _ string) (*payment.PageResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pageNum := parsePositiveInt(page, 1)
	limitNum := parsePositiveInt(limit, 10)

	all := make([]*payment.Payment, 0, len(r.payments))
	for _, p := range r.payments {
		all = append(all, clonePayment(p))
	}

	total := len(all)
	offset := (pageNum - 1) * limitNum
	if offset >= total {
		return &payment.PageResult{Items: []*payment.Payment{}, Total: total}, nil
	}

	end := offset + limitNum
	if end > total {
		end = total
	}

	return &payment.PageResult{Items: all[offset:end], Total: total}, nil
}

func (r *PaymentRepository) Save(_ context.Context, p *payment.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.payments[p.ID] = clonePayment(p)
	return nil
}

func (r *PaymentRepository) Update(_ context.Context, p *payment.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.payments[p.ID]; !ok {
		return payment.ErrNotFound
	}

	r.payments[p.ID] = clonePayment(p)
	return nil
}

func (r *PaymentRepository) FindByID(_ context.Context, id string) (*payment.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.payments[id]
	if !ok {
		return nil, payment.ErrNotFound
	}

	return clonePayment(p), nil
}

// clonePayment devolve uma cópia independente para não vazar o ponteiro guardado
// no mapa — assim mutações do chamador só persistem via Update, como num banco real.
func clonePayment(p *payment.Payment) *payment.Payment {
	cloned := *p
	return &cloned
}

func (r *PaymentRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.payments, id)
	return nil
}

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
