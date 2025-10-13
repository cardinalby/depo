package cli_handlers

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"iter"
	"os"
	"time"

	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
)

type exportFormat string

const (
	exportFormatJSON exportFormat = "json"
	exportFormatXML  exportFormat = "xml"
)

type HistoryExporter interface {
	// Run starts the CLI handler for exporting history.
	// It prompts the user for the desired export format (JSON or XML)
	// and prints the history records to standard output in the selected format.
	// The method returns io.EOF when the export is complete, or an error if any issues occur
	Run(ctx context.Context) error
}

type historyExporter struct {
	historyUsecase domain.HistoryUsecase
	timeFormat     string
}

func NewHistoryExporter(
	historyUsecase domain.HistoryUsecase,
) HistoryExporter {
	return &historyExporter{
		historyUsecase: historyUsecase,
		timeFormat:     time.ANSIC,
	}
}

func (e *historyExporter) Run(ctx context.Context) error {
	format, err := e.promptFormat(ctx)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	if err = e.printHistory(ctx, format); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
		return err
	}
	return nil
}

type historyEntryDto struct {
	Action string `json:"action"`
	Time   string `json:"time"`
}

func (e *historyExporter) promptFormat(ctx context.Context) (exportFormat, error) {
	type scanResult struct {
		input string
		err   error
	}
	resultCh := make(chan scanResult, 1)
	fmt.Print("Enter export format (json/xml): ")
	go func() {
		// not the best solution: the goroutine hangs if input is not provided (there no good portable way to
		// interrupt stdin read)
		var input string
		_, err := fmt.Scanln(&input)
		resultCh <- scanResult{input: input, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return "", fmt.Errorf("failed to read input: %w", res.err)
		}
		switch exportFormat(res.input) {
		case exportFormatJSON:
			return exportFormatJSON, nil
		case exportFormatXML:
			return exportFormatXML, nil
		default:
			return "", fmt.Errorf("unsupported format: %s", res.input)
		}
	}
}

func (e *historyExporter) printHistory(ctx context.Context, format exportFormat) error {
	recordsIter := e.historyUsecase.GetRecordsIter()

	var iterErr error
	dtosIter := func(yield func(historyEntryDto) bool) {
		for rec, err := range recordsIter {
			if err := errors.Join(err, ctx.Err()); err != nil {
				iterErr = err
				return
			}
			if !yield(historyEntryDto{
				Action: rec.Action,
				Time:   rec.Time.Format(e.timeFormat),
			}) {
				return
			}
		}
	}

	printFunc := e.getPrintFunc(format)
	printErr := printFunc(dtosIter)
	return errors.Join(iterErr, printErr)
}

func (e *historyExporter) getPrintFunc(format exportFormat) func(iter.Seq[historyEntryDto]) error {
	switch format {
	case exportFormatJSON:
		return e.printJSON
	case exportFormatXML:
		return e.printXML
	default:
		panic("unsupported format") // should not happen due to prior validation
	}
}

func (e *historyExporter) printJSON(dtosIter iter.Seq[historyEntryDto]) error {
	fmt.Println("[")
	first := true
	for dto := range dtosIter {
		if !first {
			fmt.Println(",")
		}
		first = false
		fmt.Print("  ")
		bytes, err := json.MarshalIndent(dto, "  ", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal history entry to JSON: %w", err)
		}
		fmt.Print(string(bytes))
	}
	fmt.Println("\n]")
	return nil
}

func (e *historyExporter) printXML(dtosIter iter.Seq[historyEntryDto]) error {
	encoder := xml.NewEncoder(os.Stdout)
	encoder.Indent("", "  ")
	startElem := xml.StartElement{Name: xml.Name{Local: "History"}}
	if err := encoder.EncodeToken(startElem); err != nil {
		return fmt.Errorf("failed to write XML start element: %w", err)
	}
	for dto := range dtosIter {
		entryElem := xml.StartElement{Name: xml.Name{Local: "Entry"}}
		if err := encoder.EncodeToken(entryElem); err != nil {
			return fmt.Errorf("failed to write XML entry start element: %w", err)
		}
		if err := encoder.EncodeElement(dto.Action, xml.StartElement{Name: xml.Name{Local: "Action"}}); err != nil {
			return fmt.Errorf("failed to write XML action element: %w", err)
		}
		if err := encoder.EncodeElement(dto.Time, xml.StartElement{Name: xml.Name{Local: "Time"}}); err != nil {
			return fmt.Errorf("failed to write XML time element: %w", err)
		}
		if err := encoder.EncodeToken(entryElem.End()); err != nil {
			return fmt.Errorf("failed to write XML entry end element: %w", err)
		}
	}
	if err := encoder.EncodeToken(startElem.End()); err != nil {
		return fmt.Errorf("failed to write XML end element: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return fmt.Errorf("failed to flush XML encoder: %w", err)
	}
	fmt.Println()
	return nil
}
