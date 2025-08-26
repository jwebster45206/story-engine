package state

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// GameState is the current state of a roleplay game session.
type GameState struct {
	ID                 uuid.UUID                    `json:"id"`                       // Unique ID per session
	ModelName          string                       `json:"model_name,omitempty"`     // Name of the large language model driving gameplay
	Scenario           string                       `json:"scenario,omitempty"`       // Filename of the scenario being played. Ex: "foo_scenario.json"
	SceneName          string                       `json:"scene_name,omitempty"`     // Current scene name in the scenario, if applicable
	NPCs               map[string]scenario.NPC      `json:"npcs,omitempty"`           // All NPCs in the game world
	WorldLocations     map[string]scenario.Location `json:"locations,omitempty"`      // Current locations in the game world
	Location           string                       `json:"user_location,omitempty"`  // Current location in the game world
	Inventory          []string                     `json:"user_inventory,omitempty"` // User's inventory items
	ChatHistory        []chat.ChatMessage           `json:"chat_history,omitempty"`   // Conversation history
	TurnCounter        int                          `json:"turn_counter"`             // Total number of successful chat interactions
	SceneTurnCounter   int                          `json:"scene_turn_counter"`       // Number of successful chat interactions in current scene
	Vars               map[string]string            `json:"vars,omitempty"`           // Game variables (e.g. flags, counters)
	ContingencyPrompts []string                     `json:"contingency_prompts,omitempty"`
	CreatedAt          time.Time                    `json:"created_at"`
	UpdatedAt          time.Time                    `json:"updated_at"`
}

func NewGameState(scenarioFileName string, modelName string) *GameState {
	return &GameState{
		ID:                 uuid.New(),
		ModelName:          modelName,
		Scenario:           scenarioFileName,
		ChatHistory:        make([]chat.ChatMessage, 0),
		TurnCounter:        0,
		SceneTurnCounter:   0,
		Vars:               make(map[string]string),
		ContingencyPrompts: make([]string, 0),
		NPCs:               make(map[string]scenario.NPC),
		WorldLocations:     make(map[string]scenario.Location),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
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

	if gs.SceneName != "" {
		scene, ok := s.Scenes[gs.SceneName]
		if !ok {
			return chat.ChatMessage{}, fmt.Errorf("scene %s not found in scenario %s", gs.SceneName, s.Name)
		}
		return gs.GetScenePrompt(s, &scene)
	}

	jsonState, err := json.Marshal(ToPromptState(gs))
	if err != nil {
		return chat.ChatMessage{}, err
	}

	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: fmt.Sprintf(scenario.StatePromptTemplate, s.Story, jsonState),
	}, nil
}

func (gs *GameState) GetScenePrompt(s *scenario.Scenario, scene *scenario.Scene) (chat.ChatMessage, error) {
	if gs == nil || scene == nil {
		return chat.ChatMessage{}, fmt.Errorf("game state or scene is nil")
	}

	ps := PromptState{
		NPCs:           scene.NPCs,
		WorldLocations: scene.Locations,
		Location:       gs.Location,
		Inventory:      gs.Inventory,
	}
	jsonScene, err := json.Marshal(ps)
	if err != nil {
		return chat.ChatMessage{}, err
	}

	story := s.Story
	if scene.Story != "" {
		story += "\n\n" + scene.Story
	}
	return chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: fmt.Sprintf(scenario.StatePromptTemplate, story, jsonScene),
	}, nil
}

// GetContingencyPrompts returns all applicable contingency prompts for the current game state
func (gs *GameState) GetContingencyPrompts(s *scenario.Scenario) []string {
	if gs == nil || s == nil {
		return nil
	}

	// Start with scenario-level contingency prompts
	prompts := make([]string, len(gs.ContingencyPrompts))
	copy(prompts, gs.ContingencyPrompts)

	// Add scene-level contingency prompts if in a scene
	if gs.SceneName != "" {
		if scene, ok := s.Scenes[gs.SceneName]; ok {
			prompts = append(prompts, scene.ContingencyPrompts...)
		}
	}

	return prompts
}

// GetChatMessages generates a "chat message" slice for LLM.
// This slice includes all prompts and instructions to run the game.
func (gs *GameState) GetChatMessages(requestMessage string, requestRole string, s *scenario.Scenario, count int) ([]chat.ChatMessage, error) {
	if gs == nil {
		return nil, fmt.Errorf("game state is nil")
	}

	// Translate game state to a chat prompt
	statePrompt, err := gs.GetStatePrompt(s)
	if err != nil {
		return nil, fmt.Errorf("error generating state prompt: %w", err)
	}

	// Build consolidated system prompt
	systemPrompt := scenario.BaseSystemPrompt

	// Add rating prompt
	switch s.Rating {
	case scenario.RatingG:
		systemPrompt += "\n\n" + scenario.ContentRatingG
	case scenario.RatingPG:
		systemPrompt += "\n\n" + scenario.ContentRatingPG
	case scenario.RatingPG13:
		systemPrompt += "\n\n" + scenario.ContentRatingPG13
	case scenario.RatingR:
		systemPrompt += "\n\n" + scenario.ContentRatingR
	}

	// Add state context
	systemPrompt += "\n\n" + statePrompt.Content

	// Add message instructions and contingency prompts
	systemPrompt += "\n\n" + scenario.UserPostPrompt
	contingencyPrompts := gs.GetContingencyPrompts(s)
	if len(contingencyPrompts) > 0 {
		systemPrompt += "\n\nApply the following conditional rules if their conditions are met:\n\n"
		for i, prompt := range contingencyPrompts {
			systemPrompt += fmt.Sprintf("%d. %s\n", i+1, prompt)
		}
	}

	// single system message at the beginning
	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: systemPrompt,
		},
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
		Role:    requestRole,
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

// LoadScene prepares game state with a new scene.
func (gs *GameState) LoadScene(s *scenario.Scenario, sceneName string) error {
	scene, ok := s.Scenes[sceneName]
	if !ok {
		return fmt.Errorf("scene %s not found in scenario %s", sceneName, s.Name)
	}
	gs.SceneName = sceneName

	// Reset scene turn counter when loading a new scene
	gs.SceneTurnCounter = 0

	// Initialize Vars map if it's nil
	if gs.Vars == nil {
		gs.Vars = make(map[string]string)
	}

	// Copy scene-specific elements to gamestate
	// Copy locations from scene
	if scene.Locations != nil {
		gs.WorldLocations = scene.Locations
	}

	// Copy NPCs from scene
	if scene.NPCs != nil {
		gs.NPCs = scene.NPCs
	}

	// copy stateful elements to gamestate
	for k, v := range scene.Vars {
		if _, exists := gs.Vars[k]; !exists {
			gs.Vars[k] = v
		}
	}

	return nil
}

// IncrementTurnCounters increments both the turn counter and scene turn counter
// after a successful chat interaction.
func (gs *GameState) IncrementTurnCounters() {
	gs.TurnCounter++
	gs.SceneTurnCounter++
}
