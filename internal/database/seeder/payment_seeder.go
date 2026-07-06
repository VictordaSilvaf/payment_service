package seeder

import (
	"context"
	"fmt"

	"payment_service/internal/database/factory"
	"payment_service/internal/domain/payment"
)

type PaymentSeeder struct {
	repo    payment.Repository
	factory *factory.PaymentFactory
	count   int
	fresh   bool
}

func NewPaymentSeeder(repo payment.Repository, count int, fresh bool) *PaymentSeeder {
	return &PaymentSeeder{
		repo:    repo,
		factory: factory.NewPaymentFactory(),
		count:   count,
		fresh:   fresh,
	}
}

func (s *PaymentSeeder) Name() string {
	return "PaymentSeeder"
}

func (s *PaymentSeeder) Run(ctx context.Context) error {
	if s.fresh {
		if err := s.truncate(ctx); err != nil {
			return err
		}
	}

	payments := s.factory.MakeMany(s.count)
	for _, p := range payments {
		if err := s.repo.Save(ctx, p); err != nil {
			return fmt.Errorf("seed payment %s: %w", p.ID, err)
		}
	}

	return nil
}

func (s *PaymentSeeder) truncate(ctx context.Context) error {
	type truncater interface {
		Truncate(ctx context.Context) error
	}

	t, ok := s.repo.(truncater)
	if !ok {
		return fmt.Errorf("repository does not support truncate")
	}

	return t.Truncate(ctx)
}
