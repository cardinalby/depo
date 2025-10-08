package jsonlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cardinalby/depo"
)

type logger struct {
	filepath string
	file     *os.File
}

// Logger is an example lazy JSON logger that creates the log file on the first write
// and appends JSON-encoded log entries to it
type Logger interface {
	WriteJson(dict map[string]any) error
	ReadMessages() ([]map[string]any, error)
	depo.Closer
}

const newline = byte('\n')

func NewLogger(filepath string) Logger {
	return &logger{filepath: filepath}
}

func (l *logger) WriteJson(dict map[string]any) error {
	if l.file == nil {
		file, err := os.Create(l.filepath)
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}
		l.file = file
	}
	b, err := json.Marshal(dict)
	if err != nil {
		return fmt.Errorf("failed to marshal log data: %w", err)
	}
	_, err = l.file.Write(append(b, newline))
	if err != nil {
		return fmt.Errorf("failed to write log data: %w", err)
	}
	return nil
}

func (l *logger) ReadMessages() ([]map[string]any, error) {
	b, err := os.ReadFile(l.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}
	lines := bytes.Split(b, []byte{newline})
	var msgs []map[string]any
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal(line, &msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal log line: %w", err)
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

func (l *logger) Close() {
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			log.Printf("failed to close log file: %v", err)
		}
	}
}
