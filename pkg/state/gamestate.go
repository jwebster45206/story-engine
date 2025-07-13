package state

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

// NPC represents a non-player character in the game
type NPC struct {
	Name        string `json:"name"`
	Disposition string `json:"disposition"` // e.g. "hostile", "neutral", "friendly"
	Profile     string `json:"profile"`     // short description or backstory
}

// GameState is the current state of a roleplay game session.
type GameState struct {
	ID          uuid.UUID          `json:"id"`                    // Unique ID per session
	Location    string             `json:"location,omitempty"`    // Current location in the game world
	Description string             `json:"description,omitempty"` // Description of the current scene
	Flags       map[string]bool    `json:"flags,omitempty"`
	NPCs        map[string]NPC     `json:"npcs,omitempty"`
	Inventory   []string           `json:"inventory,omitempty"`
	ChatHistory []chat.ChatMessage `json:"chat_history,omitempty"` // Conversation history
}

func NewGameState() *GameState {
	return &GameState{
		ID:          uuid.New(),
		ChatHistory: make([]chat.ChatMessage, 0),
	}
}

const PromptHistoryLimit = 10

func (gs *GameState) GetHistoryForPrompt() []byte {
	data, _ := json.Marshal(gs.ChatHistory[len(gs.ChatHistory)-PromptHistoryLimit:])
	return data
}
