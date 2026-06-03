// Package migrate applies the embedded SQL schema migrations using golang-migrate.
// Migrations are read from the embedded migrations.FS (iofs source) and applied
// over a pgx/v5 connection, so no external `migrate` binary or loose files are needed.
package migrate

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxdb "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"github.com/lunochkin/research-agent/migrations"
)

// newMigrator wires the embedded iofs source to a pgx-backed database driver.
// The caller must Close the returned *migrate.Migrate (it owns the db connection).
func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: open embedded source: %w", err)
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("migrate: open db: %w", err)
	}
	drv, err := pgxdb.WithInstance(db, &pgxdb.Config{})
	if err != nil {
		return nil, fmt.Errorf("migrate: init driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx", drv)
	if err != nil {
		return nil, fmt.Errorf("migrate: init migrator: %w", err)
	}
	return m, nil
}

// Up applies all pending migrations. A no-op (already current) is not an error.
func Up(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Down rolls back all migrations. A no-op (nothing applied) is not an error.
func Down(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}
