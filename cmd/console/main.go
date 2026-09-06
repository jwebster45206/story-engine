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
	jwtKeyFile := flag.String("jwt-private-key-file", "", "PEM file for the ES256 private key (or STORY_ENGINE_JWT_PRIVATE_KEY_FILE)")
	principalFlag := flag.String("principal", "", "principal UUID (or STORY_ENGINE_PRINCIPAL); generated if unset")
	flag.Parse()

	priv, err := loadConsolePrivateKey(*jwtKeyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	principal, generated, err := loadPrincipal(*principalFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if generated {
		fmt.Fprintf(os.Stderr, "generated principal %s (set --principal or STORY_ENGINE_PRINCIPAL to reuse)\n", principal)
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
		path = os.Getenv("STORY_ENGINE_JWT_PRIVATE_KEY_FILE")
	}
	var pemStr string
	switch {
	case path != "":
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("jwt private key file: %w", err)
		}
		pemStr = string(b)
	case os.Getenv("STORY_ENGINE_JWT_PRIVATE_KEY") != "":
		pemStr = os.Getenv("STORY_ENGINE_JWT_PRIVATE_KEY")
	default:
		return nil, fmt.Errorf("ES256 private key is required (--jwt-private-key-file, STORY_ENGINE_JWT_PRIVATE_KEY_FILE, or STORY_ENGINE_JWT_PRIVATE_KEY)")
	}
	key, err := auth.ParseES256PrivateKey(pemStr)
	if err != nil {
		return nil, fmt.Errorf("jwt private key: %w", err)
	}
	return key, nil
}

func loadPrincipal(flagVal string) (uuid.UUID, bool, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("STORY_ENGINE_PRINCIPAL"))
	}
	if raw == "" {
		return uuid.New(), true, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("principal: must be a UUID")
	}
	if id == uuid.Nil {
		return uuid.Nil, false, fmt.Errorf("principal: nil UUID is not allowed")
	}
	return id, false, nil
}

func envOr(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
