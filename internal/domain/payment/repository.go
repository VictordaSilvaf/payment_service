package payment

import "context"

// Repository is a driven port: the domain defines what it needs from persistence.
type Repository interface {
	Save(ctx context.Context, payment *Payment) error
	FindByID(ctx context.Context, id string) (*Payment, error)
	FindAll(ctx context.Context, page, limit, sort, order, search string) (*PageResult, error)
	Delete(ctx context.Context, id string) error
}
