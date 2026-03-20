package chat

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const MaxMessageLength = 255

// ChatRequest represents a chat message request made by the user
// to the story engine api.
type ChatRequest struct {
	GameStateID uuid.UUID `json:"gamestate_id"` // Unique ID for the game state
	Message     string    `json:"message"`
	Stream      bool      `json:"stream,omitempty"` // Whether to stream the response
}

// ChatResponse represents a chat message response returned by the story engine api.
// It omits GameState currently, but GameState should be updated
// within the chat handler.
type ChatResponse struct {
	GameStateID uuid.UUID     `json:"gamestate_id,omitempty"` // Unique ID for the game state
	Message     string        `json:"message,omitempty"`
	ChatHistory []ChatMessage `json:"chat_history,omitempty"` // History of chat messages
}

const (
	ChatRoleUser   = "user"      // User input
	ChatRoleAgent  = "assistant" // Narrator response
	ChatRoleSystem = "system"    // System messages
)

// ChatMessage represents a single chat message in the conversation
// This interface is defined by Ollama's API and is used to structure messages
// sent to the LLM.
type ChatMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

func (cr *ChatRequest) Validate() error {
	if cr.Message == "" {
		return fmt.Errorf("message cannot be empty")
	}
	if len(cr.Message) > MaxMessageLength {
		return fmt.Errorf("message exceeds maximum length of %d characters", MaxMessageLength)
	}
	if cr.GameStateID == uuid.Nil {
		return fmt.Errorf("game state ID cannot be empty")
	}
	return nil
}

// ImagePromptRequest is the request body for POST /v1/image-prompt.
type ImagePromptRequest struct {
	GameStateID uuid.UUID `json:"gamestate_id"`
}

// Validate checks that the request contains a valid GameStateID.
func (r *ImagePromptRequest) Validate() error {
	if r.GameStateID == uuid.Nil {
		return fmt.Errorf("gamestate_id is required")
	}
	return nil
}

// ImagePromptResponse is the response body for POST /v1/image-prompt.
type ImagePromptResponse struct {
	GameStateID uuid.UUID `json:"gamestate_id"`
	Prompt      string    `json:"prompt"`
}

// FormatWithPCName prefixes the message with the PC's name unless it already has a speaker prefix
// Returns the formatted message
func FormatWithPCName(message, pcName string) string {
	// Check if message already has a speaker prefix (format: "Name: message")
	// Accept any text before ": " within the first 50 characters as a valid speaker prefix
	if colonIndex := strings.Index(message, ": "); colonIndex > 0 && colonIndex < 50 {
		// Already has a speaker prefix, return as-is
		return message
	}

	// No speaker prefix found, add PC name
	return fmt.Sprintf("%s: %s", pcName, message)
}
