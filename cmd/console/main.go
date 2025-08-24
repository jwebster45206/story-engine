package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/state"
)

type ConsoleConfig struct {
	APIBaseURL string
	Timeout    time.Duration
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	cfg := &ConsoleConfig{
		APIBaseURL: getEnv("API_BASE_URL", "http://localhost:8080"),
		Timeout:    30 * time.Second,
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	if !testConnection(client, cfg.APIBaseURL) {
		fmt.Fprintf(os.Stderr, "Could not connect to API. Please ensure the API is running.\nTry: docker-compose up -d\n")
		os.Exit(1)
	}

	orderedNames, scenarioMap, err := listScenarios(client, cfg.APIBaseURL)
	if err != nil || len(orderedNames) == 0 {
		fmt.Fprintf(os.Stderr, "Failed to list scenarios: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Available Scenarios:")
	for i := range orderedNames {
		fmt.Printf("  %d - %s (%s)\n", i+1, orderedNames[i], scenarioMap[orderedNames[i]])
	}
	fmt.Print("\nSelect a scenario by number: ")

	var choice int
	if _, err := fmt.Scanf("%d", &choice); err != nil || choice < 1 || choice > len(orderedNames) {
		fmt.Fprintf(os.Stderr, "Invalid selection\n")
		os.Exit(1)
	}

	scenarioName := orderedNames[choice-1]
	scenarioFile := scenarioMap[scenarioName]

	gs, err := createGameState(client, cfg.APIBaseURL, scenarioFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create game state: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(NewConsoleUI(cfg, client, gs),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func testConnection(client *http.Client, baseURL string) bool {
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()
	return resp.StatusCode == http.StatusOK
}

func getGameState(client *http.Client, baseURL string, gameStateID uuid.UUID) (*state.GameState, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v1/gamestate/%s", baseURL, gameStateID))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to get game state: %s", errorResp.Error)
	}

	var gameState state.GameState
	if err := json.Unmarshal(body, &gameState); err != nil {
		return nil, fmt.Errorf("failed to parse game state response: %w", err)
	}
	return &gameState, nil
}

func createGameState(client *http.Client, baseURL string, scenarioFile string) (*state.GameState, error) {
	gameState := &state.GameState{
		Scenario: scenarioFile,
	}

	jsonData, err := json.Marshal(gameState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal game state: %w", err)
	}

	resp, err := client.Post(
		baseURL+"/v1/gamestate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to create game state: %s", errorResp.Error)
	}

	var createdGameState state.GameState
	if err := json.Unmarshal(body, &createdGameState); err != nil {
		return nil, fmt.Errorf("failed to parse game state response: %w", err)
	}

	return &createdGameState, nil
}

func listScenarios(client *http.Client, baseURL string) ([]string, map[string]string, error) {
	resp, err := client.Get(baseURL + "/v1/scenarios")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var scenarioMap map[string]string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if err := json.Unmarshal(body, &scenarioMap); err != nil {
		return nil, nil, err
	}

	var names []string
	for name := range scenarioMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, scenarioMap, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
