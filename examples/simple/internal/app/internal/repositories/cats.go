package repositories

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
)

type catsRepo struct {
	db *sql.DB
}

func NewCatsRepository(db *sql.DB) domain.CatsRepository {
	return &catsRepo{db: db}
}

func (r *catsRepo) GetAll() (cats []domain.Cat, err error) {
	rows, err := r.db.Query("SELECT id, name, age FROM cats")
	if err != nil {
		return cats, fmt.Errorf("failed to query cats: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	for rows.Next() {
		var cat domain.Cat
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Age); err != nil {
			return cats, fmt.Errorf("failed to scan cat: %w", err)
		}
		cats = append(cats, cat)
	}
	if err := rows.Err(); err != nil {
		return cats, fmt.Errorf("rows error: %w", err)
	}
	return cats, nil
}

func (r *catsRepo) Add(name string, age uint) (cat domain.Cat, err error) {
	res, err := r.db.Exec("INSERT INTO cats (name, age) VALUES (?, ?)", name, age)
	if err != nil {
		return cat, fmt.Errorf("failed to add cat: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return cat, fmt.Errorf("failed to get last insert id: %w", err)
	}
	return domain.Cat{ID: id, Name: name, Age: age}, nil
}
