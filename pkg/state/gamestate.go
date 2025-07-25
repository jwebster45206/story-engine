package state

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// GameState is the current state of a roleplay game session.
type GameState struct {
	ID       uuid.UUID `json:"id"`                 // Unique ID per session
	Scenario string    `json:"scenario,omitempty"` // Filename of the scenario being played. Ex: "foo_scenario.json"

	NPCs           map[string]scenario.NPC      `json:"world_npcs,omitempty"`
	WorldLocations map[string]scenario.Location `json:"world_locations,omitempty"` // Current locations in the game world

	Location  string   `json:"user_location,omitempty"` // Current location in the game world
	Inventory []string `json:"user_inventory,omitempty"`

	ChatHistory []chat.ChatMessage `json:"chat_history,omitempty"` // Conversation history

	Vars               map[string]string `json:"vars,omitempty"` // Game variables (e.g. flags, counters)
	ContingencyPrompts []string          `json:"contingency_prompts,omitempty"`
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

	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: fmt.Sprintf(scenario.StatePromptTemplate, s.Story, jsonState),
	}, nil
}

// GetChatMessages generates a "chat message" slice for LLM.
// This slice includes all prompts and instructions to run the game.
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
	ratingPrompt := ""
	switch s.Rating {
	case scenario.RatingG:
		ratingPrompt = "\n\n" + scenario.ContentRatingG
	case scenario.RatingPG:
		ratingPrompt = "\n\n" + scenario.ContentRatingPG
	case scenario.RatingPG13:
		ratingPrompt = "\n\n" + scenario.ContentRatingPG13
	case scenario.RatingR:
		ratingPrompt = "\n\n" + scenario.ContentRatingR
	}
	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: scenario.BaseSystemPrompt + "\n\n" + ratingPrompt,
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
