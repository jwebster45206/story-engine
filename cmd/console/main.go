package main

import (
	"crypto/ecdsa"
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
	jwtKeyFile := flag.String("jwt-key", "", "PEM file for the ES256 private key (or STORY_ENGINE_JWT_KEY)")
	principalFlag := flag.String("principal", "", "principal UUID (or STORY_ENGINE_PRINCIPAL); generated if unset")
	flag.Parse()

	priv, err := loadConsolePrivateKey(*jwtKeyFile)
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

func loadConsolePrivateKey(fileFlag string) (*ecdsa.PrivateKey, error) {
	path := fileFlag
	if path == "" {
		path = os.Getenv("STORY_ENGINE_JWT_KEY")
	}
	if path == "" {
		return nil, fmt.Errorf("ES256 private key is required (--jwt-key or STORY_ENGINE_JWT_KEY)")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jwt private key file: %w", err)
	}
	key, err := auth.ParseES256PrivateKey(string(b))
	if err != nil {
		return nil, fmt.Errorf("jwt private key: %w", err)
	}
	return key, nil
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
