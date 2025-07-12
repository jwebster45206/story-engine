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

	"github.com/jwebster45206/roleplay-agent/internal/config"
	"github.com/jwebster45206/roleplay-agent/internal/logger"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

type ConsoleConfig struct {
	APIBaseURL string
	Timeout    time.Duration
}

func main() {
	// Setup logging for console (text format for better readability)
	cfg, _ := config.Load()
	log := logger.Setup(cfg)

	// Console-specific config
	consoleConfig := &ConsoleConfig{
		APIBaseURL: getEnv("API_BASE_URL", "http://localhost:8080"),
		Timeout:    30 * time.Second,
	}

	log.Info("Starting roleplay console client",
		"api_base_url", consoleConfig.APIBaseURL)

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
		response, err := sendChatMessage(client, consoleConfig.APIBaseURL, input)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			log.Error("Failed to send chat message", "error", err, "input", input)
			continue
		}

		// Display response
		if response.Error != "" {
			fmt.Printf("⚠️  API Error: %s\n", response.Error)
		} else if response.Message != "" {
			fmt.Printf("Agent: %s\n", response.Message)
		} else {
			fmt.Println("Agent: [No response]")
		}

		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		log.Error("Error reading input", "error", err)
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

func sendChatMessage(client *http.Client, baseURL, message string) (*chat.ChatResponse, error) {
	// Prepare request
	chatReq := chat.ChatRequest{
		Message: message,
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

	// Parse response
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
