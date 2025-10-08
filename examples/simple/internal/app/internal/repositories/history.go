package repositories

import (
	"fmt"
	"time"

	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
	"github.com/cardinalby/examples/simple/internal/pkg/jsonlog"
)

type historyRepo struct {
	jsonLog jsonlog.Logger
}

func NewHistoryRepository(jsonLog jsonlog.Logger) domain.HistoryRepository {
	return &historyRepo{jsonLog: jsonLog}
}

func (r *historyRepo) GetAll() ([]domain.HistoryRec, error) {
	msgs, err := r.jsonLog.ReadMessages()
	if err != nil {
		return nil, err
	}
	records := make([]domain.HistoryRec, 0, len(msgs))
	for _, msg := range msgs {
		rec, err := r.mapToHistoryRec(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to convert log message to history record: %w", err)
		}
		records = append(records, rec)
	}
	return records, nil
}

func (r *historyRepo) Add(action string) error {
	return r.jsonLog.WriteJson(map[string]any{
		"action": action,
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (r *historyRepo) mapToHistoryRec(m map[string]any) (rec domain.HistoryRec, err error) {
	if t, ok := m["time"].(string); ok {
		rec.Time, err = time.Parse(time.RFC3339, t)
		if err != nil {
			return rec, fmt.Errorf("failed to parse time: %w", err)
		}
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
