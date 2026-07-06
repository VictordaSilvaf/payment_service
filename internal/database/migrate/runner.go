package migrate

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const defaultMigrationsPath = "file://db/migrations"

func newMigrate(dsn string) (*migrate.Migrate, error) {
	m, err := migrate.New(defaultMigrationsPath, dsn)
	if err != nil {
		return nil, fmt.Errorf("create migrator: %w", err)
	}
	return m, nil
}

func Up(dsn string) error {
	m, err := newMigrate(dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func Down(dsn string) error {
	m, err := newMigrate(dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func Steps(dsn string, n int) error {
	m, err := newMigrate(dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Steps(n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate steps: %w", err)
	}
	return nil
}

func Version(dsn string) (uint, bool, error) {
	m, err := newMigrate(dsn)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()

	return m.Version()
}
