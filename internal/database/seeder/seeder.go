package seeder

import "context"

type Seeder interface {
	Name() string
	Run(ctx context.Context) error
}

type Registry struct {
	seeders []Seeder
}

func NewRegistry(seeders ...Seeder) *Registry {
	return &Registry{seeders: seeders}
}

func (r *Registry) RunAll(ctx context.Context) error {
	for _, s := range r.seeders {
		if err := s.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
