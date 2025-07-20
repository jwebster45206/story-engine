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
	ChatRoleUser   = "user"      // Use
	ChatRoleAgent  = "assistant" // NPC
	ChatRoleSystem = "system"    // Narrator or system
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

// MetaUpdate is a compact representation of the changes made to the game state
// after processing a chat message. A MetaUpdate is much faster
// for the LLM to generate than a full game state.
type MetaUpdate struct {
	UserLocation        string   `json:"user_location,omitempty"`
	AddToInventory      []string `json:"add_to_inventory,omitempty"`
	RemoveFromInventory []string `json:"remove_from_inventory,omitempty"`
	MovedItems          []struct {
		Item       string `json:"item"`
		From       string `json:"from"`
		ToLocation string `json:"to_location,omitempty"`
		To         string `json:"to,omitempty"`
	} `json:"moved_items,omitempty"`
	UpdatedNPCs []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Location    string `json:"location"`
	} `json:"updated_npcs,omitempty"`
}
