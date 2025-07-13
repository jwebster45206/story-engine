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

// ANSI color codes
const (
	ColorReset = "\033[0m"
	ColorRed   = "\033[31m"
	ColorGreen = "\033[32m"
	ColorBlue  = "\033[36m"
)

type ConsoleConfig struct {
	APIBaseURL string
	Timeout    time.Duration
}

func printBanner() {
	banner := `   ___     ___   _       _____   ____   _         _    __   __ 
 |  _ \   / _ \ | |     | ____| |  _ \ | |       / \  |  \ / / 
 | |_) | | | | || |     |  _|   | |_) || |      / _ \  \ \/ /  
 |  _ <  | |_| || |___  | |___  |  __/ | |___  / ___ \  \  /   
 |_| \_\  \___/ |_____| |_____| |_|    |_____||_|   \_|  |_|                                                                 
       _       ____   _____  _   _   _____ 
      / \     / ___| | ____|| \ | | |_   _|
     / _ \   | |  _  |  _|  |  \| |   | |  
    / ___ \  | |_| | | |___ | |\  |   | |  
   /_/   \_\  \____| |_____||_| \_|   |_|  
                                          
`
	fmt.Print(banner)
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
func handleCommand(input string) bool {
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
	consoleConfig := &ConsoleConfig{
		APIBaseURL: getEnv("API_BASE_URL", "http://localhost:8080"),
		Timeout:    30 * time.Second,
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: consoleConfig.Timeout,
	}

	// Test connection to API
	if !testConnection(client, consoleConfig.APIBaseURL) {
		printRed("Could not connect to API. Please ensure the API is running.")
		printGreen("Try: docker-compose up -d")
		os.Exit(1)
	}

	printGreen("Connected to API successfully. ")

	// Create a new game state for this session
	gameStateID, err := createGameState(client, consoleConfig.APIBaseURL)
	if err != nil {
		printRed("Failed to create game state: " + err.Error())
		os.Exit(1)
	}

	printGreen("Game state created: " + gameStateID.String())

	// Print welcome message
	printGreen("\nWelcome to Roleplay Agent Console.")
	printGreen("Type your messages and press Enter. Type 'help' for instructions, or 'quit' to stop.")

	printDivider()
	fmt.Println("")

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
		if handleCommand(input) {
			continue
		} // Send message to API with progress dots
		response, err := sendChatMessageWithProgress(client, consoleConfig.APIBaseURL, gameStateID, input)
		if err != nil {
			printRed(err.Error())
			continue
		}

		// Print the agent response with word wrapping
		fmt.Printf("\n%sAgent:%s ", ColorGreen, ColorReset)
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
