package main

import (
	"os"

	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/cli_history_exporter"
)

func main() {
	runner := depo.NewRunner(func() {
		cli_history_exporter.App()
	})

	if err := runner.Run(nil, nil); err != nil {
		os.Exit(1)
	}
}
