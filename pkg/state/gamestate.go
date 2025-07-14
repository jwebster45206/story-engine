package state

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
	"github.com/jwebster45206/roleplay-agent/pkg/scenario"
)

// NPC represents a non-player character in the game
type NPC struct {
	Name        string `json:"name"`
	Type        string `json:"type"`                  // e.g. "villager", "guard", "merchant"
	Disposition string `json:"disposition"`           // e.g. "hostile", "neutral", "friendly"
	Description string `json:"description,omitempty"` // short description or backstory
	IsImportant bool   `json:"important,omitempty"`   // whether this NPC is important to the story
	Location    string `json:"location,omitempty"`    // where the NPC is currently located
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

// GetHistoryForPrompt truncatses the chat history to the last N messages
func (gs *GameState) GetHistoryForPrompt() []chat.ChatMessage {
	if len(gs.ChatHistory) == 0 {
		return nil
	}
	if len(gs.ChatHistory) <= PromptHistoryLimit {
		return gs.ChatHistory
	}
	// Return the last N messages for the prompt
	return gs.ChatHistory[len(gs.ChatHistory)-PromptHistoryLimit:]
}

// GetClosingPrompt returns a closing prompt for the game state
// This prompt could be customized based on the game state.
func (gs *GameState) GetClosingPrompt() chat.ChatMessage {
	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: scenario.ClosingPromptGeneral,
	}
}

func (gs *GameState) GetStatePrompt() (chat.ChatMessage, error) {
	if gs == nil {
		return chat.ChatMessage{}, fmt.Errorf("game state is nil")
	}
	jsonData, err := json.Marshal(ToPromptState(gs))
	if err != nil {
		return chat.ChatMessage{}, err
	}
	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: fmt.Sprintf("Use the following JSON game state as world context. Do not explain it.\n\nGame State:\n```json\n%s\n```", jsonData),
	}, nil
}
