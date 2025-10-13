package main

import (
	"errors"
	"os"

	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/cli_history_exporter"
)

func main() {
	app := cli_history_exporter.Init()
	err := app.Run(nil, nil)
	// app returns depo.ErrUnexpectedRunNilResult when the CLI handler completes successfully
	if err != nil && !errors.Is(err, depo.ErrUnexpectedRunNilResult) {
		os.Exit(1)
	}
}
