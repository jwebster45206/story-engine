package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
	"uuid"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jwebster45206/story-engine/internal/auth"
)

type ConsoleConfig struct {
	APIBaseURL string
	Timeout    time.Duration
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "working directory: %v\n", err)
		os.Exit(1)
	}
	priv, err := auth.LoadPrivateKey(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	raw, err := auth.Token(priv, uuid.New())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	cfg := &ConsoleConfig{
		APIBaseURL: getEnv("API_BASE_URL", "http://localhost:8080"),
		Timeout:    0, // No timeout - SSE connections are long-lived, server has 30s keepalive
	}

	client := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: auth.Bearer(raw),
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
