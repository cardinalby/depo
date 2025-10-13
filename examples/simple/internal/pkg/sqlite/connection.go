package sqlite

import (
	"context"
	"database/sql"
	"log"

	"github.com/cardinalby/depo"

	pkg_sql "github.com/cardinalby/examples/simple/internal/pkg/sql"
)
import _ "github.com/ncruces/go-sqlite3/driver"
import _ "github.com/ncruces/go-sqlite3/embed"

type db struct {
	db       *sql.DB
	filepath string
}

type Db interface {
	depo.Starter
	depo.Closer
	pkg_sql.DB
}

func NewDb(filepath string) Db {
	return &db{filepath: filepath}
}

func (s *db) Start(_ context.Context) error {
	log.Println("connecting to sqlite db")
	db, err := sql.Open("sqlite3", "file:"+s.filepath)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

func (s *db) Close() {
	log.Println("closing sqlite db")
	if err := s.db.Close(); err != nil {
		log.Println("failed to close sqlite db:", err)
	}
}

func (s *db) Conn(ctx context.Context) (*sql.Conn, error) {
	if s.db == nil {
		return nil, sql.ErrConnDone
	}
	return s.db.Conn(ctx)
}

func (s *db) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if s.db == nil {
		return nil, sql.ErrConnDone
	}
	return s.db.BeginTx(ctx, opts)
}

func (s *db) PingContext(ctx context.Context) error {
	if s.db == nil {
		return sql.ErrConnDone
	}
	return s.db.PingContext(ctx)
}

func (s *db) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.db == nil {
		return nil, sql.ErrConnDone
	}
	return s.db.ExecContext(ctx, query, args...)
}

func (s *db) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if s.db == nil {
		return nil, sql.ErrConnDone
	}
	return s.db.QueryContext(ctx, query, args...)
}

func (s *db) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	if s.db == nil {
		return nil, sql.ErrConnDone
	}
	return s.db.PrepareContext(ctx, query)
}
