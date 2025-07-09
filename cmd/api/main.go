package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/jwebster45206/roleplay-agent/internal/config"
	"github.com/jwebster45206/roleplay-agent/internal/handlers"
)

func main() {
	cfg := config.Load()

	// Set up routes using native Go http mux
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handlers.HealthHandler)

	// Start server
	addr := ":" + cfg.Port
	fmt.Printf("Starting server on %s\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
