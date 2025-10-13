package sqlite_migrations

import "github.com/cardinalby/examples/simple/internal/pkg/sql"

func GetMigrationCollection() sql.MigrationCollection {
	return sql.NewMigrationCollectionBuilder().
		Add(
			1,
			func() []string {
				return []string{
					`CREATE TABLE cats (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						name TEXT NOT NULL,
						age INTEGER NOT NULL
					);`,
				}
			},
		).GetCollection()
}
