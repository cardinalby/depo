package main

import (
	"os"

	"github.com/cardinalby/examples/simple/internal/app/cli_history_exporter"
)

func main() {
	app := cli_history_exporter.Init()

	if err := app.Run(nil, nil); err != nil {
		os.Exit(1)
	}
}
