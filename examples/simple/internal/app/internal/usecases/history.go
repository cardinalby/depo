package usecases

import (
	"errors"
	"iter"
	"time"

	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
)

type historyUsecase struct {
	historyRepo domain.HistoryRepository
}

func NewHistoryUsecase(
	historyRepo domain.HistoryRepository,
) domain.HistoryUsecase {
	return &historyUsecase{
		historyRepo: historyRepo,
	}
}

func (u *historyUsecase) GetRecordsIter() iter.Seq2[domain.HistoryRec, error] {
	return func(yield func(domain.HistoryRec, error) bool) {
		for rec, err := range u.historyRepo.GetRecordsIter() {
			if err != nil {
				yield(domain.HistoryRec{}, err)
				return
			}
			rec.Time = rec.Time.Local()
			if !yield(rec, nil) {
				return
			}
		}
	}
}

func (u *historyUsecase) Add(action string) error {
	if action == "" {
		return errors.New("action cannot be empty")
	}
	return u.historyRepo.Add(action, time.Now().UTC())
}
