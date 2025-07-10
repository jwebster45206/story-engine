package state

import (
	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

// GameState is the current state of a roleplay game session.
type GameState struct {
	ID          uuid.UUID          `json:"id"`           // Unique ID per session
	Location    string             `json:"location"`     // e.g., "stone hallway"
	Flags       map[string]bool    `json:"flags"`        // e.g., "door_locked": true
	ChatHistory []chat.ChatMessage `json:"chat_history"` // Conversation history

	// TODO: implement inventory system
	// Inventory []string        `json:"inventory"`

	// TODO: add characters and NPC stats/trust system
	// Characters map[string]CharacterState `json:"characters"`

	// TODO: Scenario context
}
