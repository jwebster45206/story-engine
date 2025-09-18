package chat

import (
	"fmt"

	"github.com/google/uuid"
)

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
	if cr.GameStateID == uuid.Nil {
		return fmt.Errorf("game state ID cannot be empty")
	}
	return nil
}
