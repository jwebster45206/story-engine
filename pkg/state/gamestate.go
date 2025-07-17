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
	Scenario    string                  `json:"scenario,omitempty"`    // Filename of the scenario being played. Ex: "foo_scenario.json"
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
		Scenario:    scenarioFileName,
		ChatHistory: make([]chat.ChatMessage, 0),
	}
}

func (gs *GameState) Validate() error {
	if gs.Scenario == "" {
		return fmt.Errorf("scenario.file_name is required")
	}
	return nil
}

// GetClosingPrompt returns a closing prompt for the game state
// This prompt could be customized based on the game state.
func (gs *GameState) GetClosingPrompt() chat.ChatMessage {
	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: scenario.ClosingPromptGeneral,
	}
}

// GetStatePrompt provides gameplay and story instructions to the LLM.
// It also provides scenario context and current game state context.
func (gs *GameState) GetStatePrompt(s *scenario.Scenario) (chat.ChatMessage, error) {
	if gs == nil {
		return chat.ChatMessage{}, fmt.Errorf("game state is nil")
	}

	gsCopy, err := gs.DeepCopy()
	if err != nil {
		return chat.ChatMessage{}, fmt.Errorf("failed to copy game state: %w", err)
	}

	// Exclude details that are not needed for the prompt
	gsCopy.Scenario = ""

	jsonState, err := json.Marshal(ToPromptState(gs))
	if err != nil {
		return chat.ChatMessage{}, err
	}

	jsonScenario, err := json.Marshal(s)
	if err != nil {
		return chat.ChatMessage{}, err
	}

	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: fmt.Sprintf("Use the following JSON as story boundaries. The user may only move to the locations defined in the scenario. Inventory may only contain the items defined in the scenario. All key NPCs for the story are pre-defined in the scenario.\n\nScenario:\n```json\n%s\n```\n\nUse the following JSON to understand current game state. \n\nGame State:\n```json\n%s\n```", jsonScenario, jsonState),
	}, nil
}

func (gs *GameState) GetChatMessages(requestMessage string, s *scenario.Scenario, count int) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("game state is nil")
	}

	// Translate game state to a chat prompt
	statePrompt, err := gs.GetStatePrompt(s)
	if err != nil {
		return nil, fmt.Errorf("error generating state prompt: %w", err)
	}

	// System prompt first
	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: scenario.BaseSystemPrompt + "\n\n" + s.Story,
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

func (gs *GameState) DeepCopy() (*GameState, error) {
	if gs == nil {
		return nil, fmt.Errorf("game state is nil")
	}

	// Marshal the original GameState to JSON
	data, err := json.Marshal(gs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal game state: %w", err)
	}

	// Unmarshal the JSON back into a new GameState instance
	var copy GameState
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal game state: %w", err)
	}

	return &copy, nil
}
