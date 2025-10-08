package domain

import "time"

type HistoryRec struct {
	Time   time.Time
	Action string
}

type HistoryRepository interface {
	GetAll() ([]HistoryRec, error)
	Add(action string) error
}
