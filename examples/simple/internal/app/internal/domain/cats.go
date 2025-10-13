package domain

import "context"

type Cat struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Age  uint   `json:"age"`
}

type CatsRepository interface {
	GetAll(ctx context.Context) ([]Cat, error)
	Add(ctx context.Context, name string, age uint) (Cat, error)
}

type CatsUsecase interface {
	GetAll(ctx context.Context) ([]Cat, error)
	Add(ctx context.Context, name string, age uint) (Cat, error)
}
