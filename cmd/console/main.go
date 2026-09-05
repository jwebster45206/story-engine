package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ConsoleConfig struct {
	APIBaseURL string
	APIKey     string
	Timeout    time.Duration
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	apiKey := os.Getenv("STORY_ENGINE_API_KEY")
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "STORY_ENGINE_API_KEY is required.\n")
		os.Exit(1)
	}

	cfg := &ConsoleConfig{
		APIBaseURL: getEnv("API_BASE_URL", "http://localhost:8080"),
		APIKey:     apiKey,
		Timeout:    0, // No timeout - SSE connections are long-lived, server has 30s keepalive
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &bearerTransport{
			base: http.DefaultTransport,
			key:  cfg.APIKey,
		},
	}

	if !testConnection(client, cfg.APIBaseURL) {
		fmt.Fprintf(os.Stderr, "Could not connect to API. Please ensure the API is running.\nTry: docker-compose up -d\n")
		os.Exit(1)
	}

	p := tea.NewProgram(NewConsoleUI(cfg, client),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
