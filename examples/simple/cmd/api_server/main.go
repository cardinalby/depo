package main

import (
	"log"

	"github.com/cardinalby/examples/simple/internal/app/api_server"
)

func main() {
	app := api_server.Init()
	if err := app.Run(nil, func() {
		log.Printf("API server is ready")
	}); err != nil {
		log.Fatalf("stopped with error: %v", err)
	}
	log.Printf("API server stopped")
}
