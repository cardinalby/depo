package repositories

import (
	"fmt"
	"iter"
	"time"

	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
	"github.com/cardinalby/examples/simple/internal/pkg/jsonlog"
)

type historyRepo struct {
	jsonLog    jsonlog.Logger
	timeFormat string
}

func NewHistoryRepository(jsonLog jsonlog.Logger) domain.HistoryRepository {
	return &historyRepo{
		jsonLog:    jsonLog,
		timeFormat: time.RFC3339,
	}
}

func (r *historyRepo) GetRecordsIter() iter.Seq2[domain.HistoryRec, error] {
	return func(yield func(domain.HistoryRec, error) bool) {
		for rawMsg, err := range r.jsonLog.GetMessagesIter() {
			if err != nil {
				yield(domain.HistoryRec{}, err)
				return
			}
			rec, err := r.mapToHistoryRec(rawMsg)
			if err != nil {
				yield(domain.HistoryRec{}, fmt.Errorf("failed to convert log message to history record: %w", err))
				return
			}
			if !yield(rec, nil) {
				return
			}
		}
	}
}

func (r *historyRepo) Add(action string, moment time.Time) error {
	return r.jsonLog.WriteJson(map[string]any{
		"action": action,
		"time":   moment.Format(r.timeFormat),
	})
}

func (r *historyRepo) mapToHistoryRec(m map[string]any) (rec domain.HistoryRec, err error) {
	if t, ok := m["time"].(string); ok {
		rec.Time, err = time.Parse(r.timeFormat, t)
		if err != nil {
			return rec, fmt.Errorf("failed to parse time: %w", err)
		}
		rec.Time = rec.Time.Local()
	} else {
		return rec, fmt.Errorf("time field is missing or not a string")
	}
	if action, ok := m["action"].(string); ok {
		rec.Action = action
	} else {
		return rec, fmt.Errorf("action field is missing or not a string")
	}
	return rec, nil
}
