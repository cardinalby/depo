package main

import (
	"log"

	"github.com/cardinalby/depo"
	"github.com/cardinalby/examples/simple/internal/app/api_server"
)

func main() {
	runner := depo.NewRunner(func() {
		api_server.App()
		// add more Runnables if needed
	})
	if err := runner.Run(nil, func() {
		log.Printf("API server app is ready")
	}); err != nil {
		log.Fatalf("API server app stopped with error: %v", err)
	}
	log.Printf("API server app stopped with no error")
}
