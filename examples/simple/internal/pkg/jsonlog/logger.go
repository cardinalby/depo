package jsonlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
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
	GetMessagesIter() iter.Seq2[map[string]any, error]
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

func (l *logger) GetMessagesIter() iter.Seq2[map[string]any, error] {
	return func(yield func(map[string]any, error) bool) {
		// read in streaming manner
		f, err := os.Open(l.filepath)
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			yield(nil, fmt.Errorf("failed to open log file: %w", err))
			return
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.Printf("failed to close log file: %v", err)
			}
		}()
		dec := json.NewDecoder(f)
		for {
			var msg map[string]any
			if err := dec.Decode(&msg); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				yield(nil, fmt.Errorf("failed to decode log entry: %w", err))
				return
			}
			if !yield(msg, nil) {
				return
			}
		}
	}
}

func (l *logger) Close() {
	log.Println("closing json log file")
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			log.Printf("failed to close log file: %v", err)
		}
	}
}
