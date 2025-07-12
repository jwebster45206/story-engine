package state

import (
	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

// GameState is the current state of a roleplay game session.
type GameState struct {
	ID          uuid.UUID          `json:"id"`           // Unique ID per session
	ChatHistory []chat.ChatMessage `json:"chat_history"` // Conversation history
	// Flags       map[string]bool    `json:"flags"`        // e.g., "door_locked": true
	// Location string `json:"location"` // e.g., "stone hallway"
	// Inventory TODO

	// Scenario TODO
}
