package main

import (
	"log"

	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/api_server"
	"github.com/cardinalby/examples/simple/internal/app/cli_history_exporter"
)

func main() {
	runner := depo.NewRunner(func() {
		// A bit of artificial example of 2 apps combined into one cmd. Can be useful for local debugging of multiple
		// services as a one executable
		api_server.App()
		cli_history_exporter.App()
	})

	if err := runner.Run(nil, func() {
		log.Printf("Combined app is ready")
	}); err != nil {
		log.Fatalf("Combined app stopped with error: %v", err)
	}
}
