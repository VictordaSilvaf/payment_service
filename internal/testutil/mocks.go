package testutil

import (
	"context"
	"sync"

	"payment_service/internal/application/idempotency"
	"payment_service/internal/domain/payment"
)

type MockPublisher struct {
	Err    error
	Called bool
	Last   *payment.Payment
}

func (m *MockPublisher) PublishCreated(_ context.Context, p *payment.Payment) error {
	m.Called = true
	m.Last = p
	return m.Err
}

type MemoryIdempotencyRepo struct {
	mu    sync.Mutex
	locks map[string]bool
	data  map[string]idempotency.CachedResponse
}

func NewMemoryIdempotencyRepo() *MemoryIdempotencyRepo {
	return &MemoryIdempotencyRepo{
		locks: make(map[string]bool),
		data:  make(map[string]idempotency.CachedResponse),
	}
}

func (r *MemoryIdempotencyRepo) Lock(_ context.Context, key string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.locks[key] {
		return false, nil
	}
	r.locks[key] = true
	return true, nil
}

func (r *MemoryIdempotencyRepo) Unlock(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.locks, key)
	return nil
}

func (r *MemoryIdempotencyRepo) Save(_ context.Context, key string, response idempotency.CachedResponse) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = response
	return nil
}

func (r *MemoryIdempotencyRepo) Find(_ context.Context, key string) (idempotency.CachedResponse, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	response, ok := r.data[key]
	return response, ok, nil
}

type ErrorPaymentRepository struct {
	SaveErr error
}

func (r *ErrorPaymentRepository) Save(_ context.Context, _ *payment.Payment) error {
	return r.SaveErr
}

func (r *ErrorPaymentRepository) Update(_ context.Context, _ *payment.Payment) error {
	return r.SaveErr
}

func (r *ErrorPaymentRepository) FindByID(_ context.Context, _ string) (*payment.Payment, error) {
	return nil, payment.ErrNotFound
}

func (r *ErrorPaymentRepository) FindAll(_ context.Context, _, _, _, _, _ string) (*payment.PageResult, error) {
	return nil, r.SaveErr
}

func (r *ErrorPaymentRepository) Delete(_ context.Context, _ string) error {
	return nil
}
