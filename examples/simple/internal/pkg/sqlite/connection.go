package sqlite

import (
	"context"
	"database/sql"
	"log"

	"github.com/cardinalby/depo"
)
import _ "github.com/ncruces/go-sqlite3/driver"
import _ "github.com/ncruces/go-sqlite3/embed"

type db struct {
	db  *sql.DB
	dsn string
}

type Db interface {
	depo.Starter
	depo.Closer
	// GetDB returns the underlying *sql.DB instance that is valid once Start has completed
	// and until Close is called
	GetDB() *sql.DB
}

func NewDb(dsn string) Db {
	return &db{dsn: dsn}
}

func (s *db) Start(_ context.Context) error {
	db, err := sql.Open("sqlite3", s.dsn)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

func (s *db) Close() {
	if err := s.db.Close(); err != nil {
		log.Println("failed to close sqlite db:", err)
	}
}

func (s *db) GetDB() *sql.DB {
	return s.db
}
