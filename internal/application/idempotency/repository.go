package idempotency

import "context"

type Repository interface {
	Lock(ctx context.Context, key string) (bool, error)
	Unlock(ctx context.Context, key string) error
	Save(ctx context.Context, key string, response CachedResponse) error
	Find(ctx context.Context, key string) (CachedResponse, bool, error)
}
