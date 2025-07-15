package state

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
	"github.com/jwebster45206/roleplay-agent/pkg/scenario"
)

// GameState is the current state of a roleplay game session.
type GameState struct {
	ID          uuid.UUID               `json:"id"`                    // Unique ID per session
	Scenario    scenario.Scenario       `json:"scenario,omitempty"`    // Name of the scenario being played
	Location    string                  `json:"location,omitempty"`    // Current location in the game world
	Description string                  `json:"description,omitempty"` // Description of the current scene
	Flags       map[string]bool         `json:"flags,omitempty"`
	NPCs        map[string]scenario.NPC `json:"npcs,omitempty"`
	Inventory   []string                `json:"inventory,omitempty"`
	ChatHistory []chat.ChatMessage      `json:"chat_history,omitempty"` // Conversation history
}

func NewGameState(scenarioFileName string) *GameState {
	return &GameState{
		ID:          uuid.New(),
		Scenario:    scenario.Scenario{FileName: scenarioFileName},
		ChatHistory: make([]chat.ChatMessage, 0),
	}
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

func (gs *GameState) GetChatMessages(requestMessage string, count int) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("game state is nil")
	}

	// Translate game state to a chat prompt
	statePrompt, err := gs.GetStatePrompt()
	if err != nil {
		return nil, fmt.Errorf("error generating state prompt: %w", err)
	}

	// System prompt first
	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: scenario.BaseSystemPrompt + "\n\n" + scenario.PirateScenarioPrompt,
		},
		statePrompt, // game state context json
	}

	// Add chat history from game state
	if len(gs.ChatHistory) > 0 {
		if len(gs.ChatHistory) <= count {
			messages = append(messages, gs.ChatHistory...)
		} else {
			messages = append(messages, gs.ChatHistory[len(gs.ChatHistory)-count:]...)
		}
	}

	// Add user message
	messages = append(messages, chat.ChatMessage{
		Role:    chat.ChatRoleUser,
		Content: requestMessage,
	})

	// Add closing prompt
	messages = append(messages, gs.GetClosingPrompt())

	return messages, nil
}
