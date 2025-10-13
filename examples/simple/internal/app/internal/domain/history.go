package domain

import (
	"iter"
	"time"
)

type HistoryRec struct {
	Time   time.Time
	Action string
}

type HistoryRepository interface {
	GetRecordsIter() iter.Seq2[HistoryRec, error]
	Add(action string, moment time.Time) error
}

type HistoryUsecase interface {
	GetRecordsIter() iter.Seq2[HistoryRec, error]
	Add(action string) error
}
