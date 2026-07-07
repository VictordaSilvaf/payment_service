package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const (
	defaultMigrationsPath = "file://db/migrations"
	migrationsDir         = "db/migrations"
)

var (
	migrationFileRe = regexp.MustCompile(`^(\d+)_.*\.(up|down)\.sql$`)
	nonAlnumRe      = regexp.MustCompile(`[^a-z0-9]+`)
)

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

// Create gera um par de arquivos de migration (up/down) em db/migrations,
// usando numeração sequencial com zero-padding (ex.: 000003_create_outbox.up.sql).
// Retorna os caminhos dos arquivos criados.
func Create(name string) ([]string, error) {
	slug := slugify(name)
	if slug == "" {
		return nil, errors.New("migration name is required")
	}

	next, err := nextSequence()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create migrations dir: %w", err)
	}

	created := make([]string, 0, 2)
	for _, direction := range []string{"up", "down"} {
		filename := fmt.Sprintf("%06d_%s.%s.sql", next, slug, direction)
		path := filepath.Join(migrationsDir, filename)
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			return created, fmt.Errorf("write %s: %w", path, err)
		}
		created = append(created, path)
	}

	return created, nil
}

// nextSequence descobre o próximo número de versão inspecionando os arquivos existentes.
func nextSequence() (int, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 1, nil
		}
		return 0, fmt.Errorf("read migrations dir: %w", err)
	}

	max := 0
	for _, entry := range entries {
		matches := migrationFileRe.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		n, convErr := strconv.Atoi(matches[1])
		if convErr != nil {
			continue
		}
		if n > max {
			max = n
		}
	}

	return max + 1, nil
}

// slugify normaliza o nome para snake_case seguro em nome de arquivo.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlnumRe.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}
