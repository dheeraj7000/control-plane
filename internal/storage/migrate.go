// Package storage holds the Postgres-backed Repository implementations
// for every domain aggregate — the swap-in the Milestone 1 architecture
// decision promised: repository *interfaces* live next to their owning
// domain package (workflow.Repository, execution.Repository,
// agent.Repository, budget.Repository, events.Store); this package
// only implements them, using the domain packages' Restore
// constructors (added this milestone) to reconstitute aggregates from
// rows without going through their "fresh creation" constructors.
//
// Every earlier milestone developed and tested against each package's
// InMemoryRepository — those remain the real implementations for tests
// and are still exported; internal/app now wires these Postgres ones
// for production use.
package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies every pending migration in migrations/ to
// databaseURL. Safe to call every time the process starts — a
// already-up-to-date database is a no-op (migrate.ErrNoChange is not
// treated as an error).
func Migrate(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("storage: open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("storage: init migration driver: %w", err)
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("storage: load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("storage: init migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("storage: apply migrations: %w", err)
	}
	return nil
}
