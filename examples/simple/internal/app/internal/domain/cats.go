package domain

type Cat struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Age  uint   `json:"age"`
}

type CatsRepository interface {
	GetAll() ([]Cat, error)
	Add(name string, age uint) (Cat, error)
}

type CatsUsecase interface {
	GetAll() ([]Cat, error)
	Add(name string, age uint) (Cat, error)
}
