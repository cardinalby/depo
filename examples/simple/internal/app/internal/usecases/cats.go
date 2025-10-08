package usecases

import (
	"fmt"

	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
)

type catsUsecase struct {
	catsRepo    domain.CatsRepository
	historyRepo domain.HistoryRepository
}

func NewCatsUsecase(
	catsRepo domain.CatsRepository,
	historyRepo domain.HistoryRepository,
) domain.CatsUsecase {
	return &catsUsecase{
		catsRepo:    catsRepo,
		historyRepo: historyRepo,
	}
}

func (u *catsUsecase) GetAll() (cats []domain.Cat, err error) {
	cats, err = u.catsRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get all cats: %w", err)
	}
	if err := u.historyRepo.Add("Fetched all cats"); err != nil {
		return cats, fmt.Errorf("failed to log history: %w", err)
	}
	return cats, nil
}

func (u *catsUsecase) Add(name string, age uint) (cat domain.Cat, err error) {
	cat, err = u.catsRepo.Add(name, age)
	if err != nil {
		return cat, fmt.Errorf("failed to add cat: %w", err)
	}
	if err := u.historyRepo.Add(fmt.Sprintf("Added cat with ID %d", cat.ID)); err != nil {
		return cat, err
	}
	return cat, nil
}
