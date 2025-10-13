package main

import (
	"log"

	"github.com/cardinalby/examples/simple/internal/app/api_server"
)

func main() {
	app := api_server.Init()
	if err := app.Run(nil, func() {
		log.Printf("API server app is ready")
	}); err != nil {
		log.Fatalf("API server app stopped with error: %v", err)
	}
	log.Printf("API server app stopped with no error")
}
