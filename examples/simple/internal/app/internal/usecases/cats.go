package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
)

type catsUsecase struct {
	catsRepo       domain.CatsRepository
	historyUsecase domain.HistoryUsecase
	timeout        time.Duration
}

func NewCatsUsecase(
	catsRepo domain.CatsRepository,
	historyUsecase domain.HistoryUsecase,
	timeout time.Duration,
) domain.CatsUsecase {
	return &catsUsecase{
		catsRepo:       catsRepo,
		historyUsecase: historyUsecase,
		timeout:        timeout,
	}
}

func (u *catsUsecase) GetAll(ctx context.Context) (cats []domain.Cat, err error) {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	cats, err = u.catsRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all cats: %w", err)
	}
	if err := u.historyUsecase.Add("Fetched all cats"); err != nil {
		return cats, fmt.Errorf("failed to log history: %w", err)
	}
	return cats, nil
}

func (u *catsUsecase) Add(ctx context.Context, name string, age uint) (cat domain.Cat, err error) {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()

	cat, err = u.catsRepo.Add(ctx, name, age)
	if err != nil {
		return cat, fmt.Errorf("failed to add cat: %w", err)
	}
	if err := u.historyUsecase.Add(fmt.Sprintf("Added cat with ID %d", cat.ID)); err != nil {
		return cat, fmt.Errorf("failed to log history: %w", err)
	}
	return cat, nil
}
