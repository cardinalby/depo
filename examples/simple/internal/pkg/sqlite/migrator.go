package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/cardinalby/depo"
	pkg_sql "github.com/cardinalby/examples/simple/internal/pkg/sql"
)

type migrator struct {
	collection pkg_sql.MigrationCollection
	db         pkg_sql.DB
}

type Migrator interface {
	depo.Starter
}

func NewMigrator(
	db pkg_sql.DB,
	collection pkg_sql.MigrationCollection,
) Migrator {
	return &migrator{
		collection: collection,
		db:         db,
	}
}

func (m *migrator) Start(ctx context.Context) (err error) {
	log.Println("migrating sqlite database")
	// normally you would use go-migrate or similar tool to manage migrations.
	return m.performInTx(ctx, func(tx *sql.Tx) error {
		currVersion, err := m.initMigratorTable(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to init schema version table: %w", err)
		}
		// read all sql files from the embedded filesystem and execute them in order.
		entries, err := m.collection.GetEntries(currVersion)
		if err != nil {
			return fmt.Errorf("failed to get migration entries: %w", err)
		}
		if len(entries) == 0 {
			return nil
		}
		migratedToVersion := currVersion

		for _, entry := range entries {
			for _, statement := range entry.Statements {
				if _, err = tx.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("failed to execute `%s` statement for version %d: %w",
						statement, entry.Version, err,
					)
				}
			}
			migratedToVersion = entry.Version
		}

		if err = m.updateLastMigrationVersion(ctx, tx, migratedToVersion); err != nil {
			return fmt.Errorf("failed to update version: %w", err)
		}

		return nil
	})
}

func (m *migrator) performInTx(
	ctx context.Context,
	f func(tx *sql.Tx) error,
) (err error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("failed to rollback transaction: %v", rbErr)
			}
		} else if cmErr := tx.Commit(); cmErr != nil {
			err = fmt.Errorf("failed to commit transaction: %w", cmErr)
		}
	}()
	if err = f(tx); err != nil {
		return fmt.Errorf("failed to perform operation in transaction: %w", err)
	}
	return nil
}

func (m *migrator) initMigratorTable(ctx context.Context, tx *sql.Tx) (currVersion uint32, err error) {
	_, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _migrator (version INTEGER NOT NULL)`)
	if err != nil {
		return 0, fmt.Errorf("failed to create _schema_version table: %w", err)
	}
	err = tx.QueryRowContext(ctx, `SELECT version FROM _migrator LIMIT 1`).Scan(&currVersion)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to get current schema version: %w", err)
	}
	return currVersion, nil
}

func (m *migrator) updateLastMigrationVersion(ctx context.Context, tx *sql.Tx, newVersion uint32) (err error) {
	res, err := tx.ExecContext(ctx, `UPDATE _migrator SET version = ?`, newVersion)
	if err != nil {
		return fmt.Errorf("failed to perform update: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO _migrator (version) VALUES (?)`, newVersion)
		if err != nil {
			return fmt.Errorf("failed to perform insert: %w", err)
		}
	}
	return nil
}

func (m *migrator) executeFileStatements(ctx context.Context, tx *sql.Tx, sqlBytes []byte) (err error) {
	statements := bytes.Split(sqlBytes, []byte{';'})
	for _, stmt := range statements {
		stmt = bytes.TrimSpace(stmt)
		if len(stmt) == 0 {
			continue
		}
		if _, err = tx.ExecContext(ctx, string(stmt)); err != nil {
			return fmt.Errorf("failed to execute statement %q: %w", stmt, err)
		}
	}
	return nil
}
