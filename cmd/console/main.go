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
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// ANSI color codes
const (
	ColorReset = "\033[0m"
	ColorRed   = "\033[31m"
	ColorGreen = "\033[32m"
	ColorBlue  = "\033[36m"
	AgentName  = "Narrator"

	ScenarioFile = "pirate.json" // TODO: Allow user to select scenario file
)

type ConsoleConfig struct {
	APIBaseURL string
	Timeout    time.Duration
}

func printBanner() {
	fmt.Println("Welcome to STORY ENGINE console!\nThis is a text-based adventure game.  Follow the prompts to solve the scenario. \nYou can type commands like 'help' for instructions, or 'quit' to exit.")
	printDivider()
	println()
}

func printGreen(text string) {
	fmt.Printf("%s%s%s\n", ColorGreen, text, ColorReset)
}

func printRed(text string) {
	fmt.Printf("%s%s%s\n", ColorRed, text, ColorReset)
}

func printBlue(text string) {
	fmt.Printf("%s%s%s", ColorBlue, text, ColorReset)
}

func printDivider() {
	fmt.Println(strings.Repeat("-", 50))
}

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

func wrapText(text string, width int) string {
	// Split by lines first to preserve intentional line breaks
	lines := strings.Split(text, "\n")
	var wrappedLines []string

	for _, line := range lines {
		// If line is empty or only whitespace, preserve it as-is
		if strings.TrimSpace(line) == "" {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		// If line fits within width, keep it as-is
		if len(line) <= width {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		// Line is too long, need to wrap it
		words := strings.Fields(line)
		if len(words) == 0 {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		var currentLine strings.Builder

		for _, word := range words {
			// Check if adding this word would exceed the width
			testLine := currentLine.String()
			if testLine != "" {
				testLine += " "
			}
			testLine += word

			if len(testLine) <= width {
				// Word fits on current line
				if currentLine.Len() > 0 {
					currentLine.WriteString(" ")
				}
				currentLine.WriteString(word)
			} else {
				// Word doesn't fit, finish current line and start new one
				if currentLine.Len() > 0 {
					wrappedLines = append(wrappedLines, currentLine.String())
					currentLine.Reset()
				}
				currentLine.WriteString(word)
			}
		}

		// Add the last line if it has content
		if currentLine.Len() > 0 {
			wrappedLines = append(wrappedLines, currentLine.String())
		}
	}

	return strings.Join(wrappedLines, "\n")
}

func printWrapped(text string) {
	wrapped := wrapText(text, 80) // 80 characters is a good default for most terminals
	fmt.Print(wrapped)
}

func printHelp() {
	printGreen("=== ROLEPLAY AGENT COMMANDS ===")
	fmt.Println()
	printGreen("Game Commands:")
	fmt.Println("  help     - Show this help message")
	fmt.Println("  quit     - Exit the game")
	fmt.Println()
	printGreen("How to Play:")
	fmt.Println("  - Type your message and press Enter to interact with the AI agent")
	fmt.Println("  - The agent will respond in character based on the roleplay scenario")
	fmt.Println("  - Use natural language to describe actions, ask questions, or continue the story")
	fmt.Println()
	printGreen("Tips:")
	fmt.Println("  - Be descriptive in your actions for more immersive responses")
	fmt.Println("  - Don't break character - stay in the role you choose")
	fmt.Println()
}

func confirmQuit() bool {
	printGreen("Are you sure you want to quit? (y/n)")
	println("")
	printBlue("Answer: ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		// If there's an error reading input, default to not quitting
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// handleCommand processes user input for special commands
// Returns true if a command was handled, false if input should be sent to agent
func handleCommand(cfg *ConsoleConfig, input string, gsID uuid.UUID, client *http.Client) bool {
	command := strings.ToLower(strings.TrimSpace(input))

	switch command {
	case "help", "h":
		printHelp()
		return true

	case "quit", "q", "exit":
		if confirmQuit() {
			os.Exit(0)
		}
		return true
	}

	// Get GameState from API
	var gs *state.GameState
	if (command != "help" && command != "h") && (command != "quit" && command != "q" && command != "exit") {
		// Only fetch game state if not a command that doesn't require it
		var err error
		gs, err = getGameState(client, cfg.APIBaseURL, gsID)
		if err != nil {
			printRed("Failed to get game state: " + err.Error())
			return true
		}
	}

	// Get Scenario from API
	var s *scenario.Scenario
	s, err := getScenario(client, cfg.APIBaseURL, gs.Scenario)
	if err != nil {
		printRed("Failed to get scenario: " + err.Error())
		return true
	}

	switch command {

	case "i", "inventory":
		if len(gs.Inventory) == 0 {
			printGreen(AgentName + ": Your inventory is empty.")
			println("")
		} else {
			items := strings.Join(gs.Inventory, "\n- ")
			printGreen(AgentName + ": Your inventory contains:")
			printGreen("- " + items)
			println("")
		}
		return true

	case "l", "location":
		if gs.Location == "" {
			printGreen(AgentName + ": You are in an unknown location.")
			println("")
			return true
		}
		if s, ok := s.Locations[gs.Location]; ok {
			printGreen(fmt.Sprintf("%s: %s, %s", AgentName, gs.Location, s))
		} else {
			printGreen(fmt.Sprintf("%s: %s", AgentName, gs.Location))
		}
		println("")
		return true

	case "gs":
		jsonData, err := json.MarshalIndent(gs, "", "  ")
		if err != nil {
			printRed("Failed to serialize game state: " + err.Error())
			return true
		} else {
			printGreen("Game state:")
			printWrapped(string(jsonData))
			println("")
			return true
		}

	default:
		return false
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	clearScreen()
	printDivider()
	printBanner()

	// Console-specific config
	cfg := &ConsoleConfig{
		APIBaseURL: getEnv("API_BASE_URL", "http://localhost:8080"),
		Timeout:    30 * time.Second,
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	// Test connection to API
	if !testConnection(client, cfg.APIBaseURL) {
		printRed("Could not connect to API. Please ensure the API is running.")
		printGreen("Try: docker-compose up -d")
		os.Exit(1)
	}

	printGreen("Connected to API successfully. ")

	// Create a new game state for this session
	gs, err := createGameState(client, cfg.APIBaseURL)
	if err != nil {
		printRed("Failed to create game state: " + err.Error())
		os.Exit(1)
	}
	printGreen("Game state created: " + gs.ID.String())

	// Print welcome message
	printGreen("\nWelcome to Roleplay Agent Console.")
	printGreen("Type your messages and press Enter. Type 'help' for instructions, or 'quit' to stop.")

	printDivider()
	fmt.Println("")

	// Print initial scenario description
	// It should be the first item in the chat history
	if len(gs.ChatHistory) > 0 {
		printWrapped(gs.ChatHistory[0].Content)
		fmt.Println("")
		fmt.Println("")
	}

	// Main chat loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		printBlue("You: ")

		// Read user input
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		// Check for empty input
		if input == "" {
			continue
		}

		// Handle special commands first
		if handleCommand(cfg, input, gs.ID, client) {
			continue
		}

		// Send message to API with progress dots
		response, err := sendChatMessageWithProgress(client, cfg.APIBaseURL, gs.ID, input)
		if err != nil {
			printRed(err.Error())
			continue
		}

		// Print the narrator response
		fmt.Printf("\n%sNarrator:%s ", ColorGreen, ColorReset)
		printWrapped(response.Message)
		fmt.Println()
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		printRed("Error reading input: " + err.Error())
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

func getGameState(client *http.Client, baseURL string, gameStateID uuid.UUID) (*state.GameState, error) {
	resp, err := client.Get(fmt.Sprintf("%s/gamestate/%s", baseURL, gameStateID))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

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

func createGameState(client *http.Client, baseURL string) (*state.GameState, error) {
	// Create a new game state
	gameState := &state.GameState{
		Scenario: ScenarioFile,
	}

	jsonData, err := json.Marshal(gameState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal game state: %w", err)
	}

	// Send POST request to create game state
	resp, err := client.Post(
		baseURL+"/gamestate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for potential error messages
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if request was successful
	if resp.StatusCode != http.StatusCreated {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to create game state: %s", errorResp.Error)
	}

	// Parse successful response to get the created game state
	var createdGameState state.GameState
	if err := json.Unmarshal(body, &createdGameState); err != nil {
		return nil, fmt.Errorf("failed to parse game state response: %w", err)
	}

	return &createdGameState, nil
}

func sendChatMessageWithProgress(client *http.Client, baseURL string, gameStateID uuid.UUID, message string) (*chat.ChatResponse, error) {
	// Prepare request
	chatReq := chat.ChatRequest{
		GameStateID: gameStateID,
		Message:     message,
	}

	jsonData, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create a channel to receive the result
	resultChan := make(chan struct {
		response *chat.ChatResponse
		err      error
	})

	// Start the request in a goroutine
	go func() {
		// Send POST request
		resp, err := client.Post(
			baseURL+"/chat",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			resultChan <- struct {
				response *chat.ChatResponse
				err      error
			}{nil, fmt.Errorf("failed to send request: %w", err)}
			return
		}
		defer resp.Body.Close()

		// Read response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resultChan <- struct {
				response *chat.ChatResponse
				err      error
			}{nil, fmt.Errorf("failed to read response: %w", err)}
			return
		}

		// Check if request was successful
		if resp.StatusCode != http.StatusOK {
			var errorResp ErrorResponse
			if err := json.Unmarshal(body, &errorResp); err != nil {
				resultChan <- struct {
					response *chat.ChatResponse
					err      error
				}{nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))}
				return
			}
			resultChan <- struct {
				response *chat.ChatResponse
				err      error
			}{nil, fmt.Errorf("chat request failed: %s", errorResp.Error)}
			return
		}

		// Parse successful response
		var chatResp chat.ChatResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			resultChan <- struct {
				response *chat.ChatResponse
				err      error
			}{nil, fmt.Errorf("failed to parse response: %w", err)}
			return
		}

		resultChan <- struct {
			response *chat.ChatResponse
			err      error
		}{&chatResp, nil}
	}()

	// Show progress dots while waiting
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case result := <-resultChan:
			// Clear the current line and return to beginning
			fmt.Print("\r\033[K")
			return result.response, result.err
		case <-ticker.C:
			fmt.Print(".")
		}
	}
}

func getScenario(client *http.Client, baseURL string, filename string) (*scenario.Scenario, error) {
	resp, err := client.Get(fmt.Sprintf("%s/scenario/%s", baseURL, filename))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to get scenario: %s", errorResp.Error)
	}

	var s scenario.Scenario
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("failed to parse scenario response: %w", err)
	}
	return &s, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
