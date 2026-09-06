package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
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
	principalFlag := flag.String("principal", "", "principal UUID (or STORY_ENGINE_PRINCIPAL); generated if unset")
	flag.Parse()

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
	principal, err := loadPrincipal(*principalFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	cfg := &ConsoleConfig{
		APIBaseURL: envOr("API_BASE_URL", "http://localhost:8080"),
		Timeout:    0, // No timeout - SSE connections are long-lived, server has 30s keepalive
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &bearerTransport{
			base:      http.DefaultTransport,
			key:       priv,
			principal: principal,
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

func loadPrincipal(flagVal string) (uuid.UUID, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("STORY_ENGINE_PRINCIPAL"))
	}
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("principal: must be a UUID")
	}
	if id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("principal: nil UUID is not allowed")
	}
	return id, nil
}

func envOr(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
