package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
	"github.com/jwebster45206/roleplay-agent/pkg/state"
)

type ConsoleConfig struct {
	APIBaseURL string
	Timeout    time.Duration
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {

	// Console-specific config
	consoleConfig := &ConsoleConfig{
		APIBaseURL: getEnv("API_BASE_URL", "http://localhost:8080"),
		Timeout:    30 * time.Second,
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: consoleConfig.Timeout,
	}

	// Print welcome message
	fmt.Println("🎭 Welcome to Roleplay Agent Console!")
	fmt.Println("Type your messages and press Enter. Type 'quit' or 'exit' to stop.")
	fmt.Println("Connecting to API at:", consoleConfig.APIBaseURL)
	fmt.Println(strings.Repeat("-", 50))

	// Test connection to API
	if !testConnection(client, consoleConfig.APIBaseURL) {
		fmt.Println("❌ Could not connect to API. Please ensure the API is running.")
		fmt.Println("💡 Try: docker-compose up -d")
		os.Exit(1)
	}

	fmt.Println("✅ Connected to API successfully!")

	// Create a new game state for this session
	gameStateID, err := createGameState(client, consoleConfig.APIBaseURL)
	if err != nil {
		fmt.Printf("❌ Failed to create game state: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🎮 Game state created: %s\n", gameStateID)
	fmt.Println(strings.Repeat("-", 50))

	// Main chat loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")

		// Read user input
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		// Check for exit commands
		if input == "quit" || input == "exit" || input == "" {
			fmt.Println("👋 Goodbye!")
			break
		}

		// Send message to API
		response, err := sendChatMessage(client, consoleConfig.APIBaseURL, gameStateID, input)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			continue
		}
		fmt.Printf("Agent: %s\n", response.Message)
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("❌ Error reading input: %v\n", err)
	}
}

func testConnection(client *http.Client, baseURL string) bool {
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func createGameState(client *http.Client, baseURL string) (uuid.UUID, error) {
	// Create a new game state
	gameState := &state.GameState{
		ID:          uuid.New(),
		ChatHistory: []chat.ChatMessage{},
	}

	jsonData, err := json.Marshal(gameState)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshal game state: %w", err)
	}

	// Send POST request to create game state
	resp, err := client.Post(
		baseURL+"/gamestate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for potential error messages
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if request was successful
	if resp.StatusCode != http.StatusCreated {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return uuid.Nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return uuid.Nil, fmt.Errorf("failed to create game state: %s", errorResp.Error)
	}

	// Parse successful response to get the created game state
	var createdGameState state.GameState
	if err := json.Unmarshal(body, &createdGameState); err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse game state response: %w", err)
	}

	return createdGameState.ID, nil
}

func sendChatMessage(client *http.Client, baseURL string, gameStateID uuid.UUID, message string) (*chat.ChatResponse, error) {
	// Prepare request
	chatReq := chat.ChatRequest{
		GameStateID: gameStateID,
		Message:     message,
	}

	jsonData, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send POST request
	resp, err := client.Post(
		baseURL+"/chat",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if request was successful
	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("chat request failed: %s", errorResp.Error)
	}

	// Parse successful response
	var chatResp chat.ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
