package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/cardinalby/depo"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

type migrator struct {
	db     *sql.DB
	fs     fs.FS
	dbName string
}

type Migrator interface {
	depo.Starter
}

func NewMigrator(
	db *sql.DB,
	fs fs.FS,
	dbName string,
) Migrator {
	return &migrator{
		db:     db,
		fs:     fs,
		dbName: dbName,
	}
}

func (m *migrator) Start(_ context.Context) error {
	source, err := iofs.New(m.fs, ".")
	if err != nil {
		return fmt.Errorf("failed to create iofs source driver: %w", err)
	}
	dbInstance, err := sqlite3.WithInstance(m.db, nil)
	if err != nil {
		return fmt.Errorf("failed to create sqlite3 database driver: %w", err)
	}

	goMigrate, err := migrate.NewWithInstance(
		"sqlite_files",
		source,
		m.dbName,
		dbInstance,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	if err := goMigrate.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run up migrations: %w", err)
	}
	return nil
}
