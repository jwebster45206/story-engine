package state

import (
	"encoding/json"
	"fmt"
	"maps"
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
	IsEnded            bool                         `json:"is_ended"`                 // true when the game is over
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
		return chat.ChatMessage{}, fmt.Errorf("game state or scene is nil")
	}

	var scene *scenario.Scene
	if gs.SceneName != "" {
		sc, ok := s.Scenes[gs.SceneName]
		if !ok {
			return chat.ChatMessage{}, fmt.Errorf("scene %s not found in scenario %s", gs.SceneName, s.Name)
		}
		scene = &sc
	}

	ps := ToPromptState(gs)
	jsonScene, err := json.Marshal(ps)
	if err != nil {
		return chat.ChatMessage{}, err
	}

	story := s.Story
	if scene != nil && scene.Story != "" {
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
	systemPrompt += "\n\nContent Rating: " + s.Rating
	switch s.Rating {
	case scenario.RatingG:
		systemPrompt += "- " + scenario.ContentRatingG
	case scenario.RatingPG:
		systemPrompt += "- " + scenario.ContentRatingPG
	case scenario.RatingPG13:
		systemPrompt += "- " + scenario.ContentRatingPG13
	case scenario.RatingR:
		systemPrompt += "- " + scenario.ContentRatingR
	}

	// Add state context
	systemPrompt += "\n\n" + statePrompt.Content

	// Add message instructions and contingency prompts
	systemPrompt += "\n\n" + scenario.UserPostPrompt

	if gs.IsEnded {
		// if the game is over, add the end prompt
		systemPrompt += "\n\n" + scenario.GameEndSystemPrompt
		if s.GameEndPrompt != "" {
			systemPrompt += "\n\n" + s.GameEndPrompt
		}
	} else {
		// contingency prompts otherwise
		contingencyPrompts := gs.GetContingencyPrompts(s)
		if len(contingencyPrompts) > 0 {
			systemPrompt += "\n\nApply the following conditional rules if their conditions are met:\n\n"
			for i, prompt := range contingencyPrompts {
				systemPrompt += fmt.Sprintf("%d. %s\n", i+1, prompt)
			}
		}
	}

	// start building chat messages, starting with
	// the full system prompt
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

// LoadScene prepares game state with a new scene:
// - loads new locations, NPCs, and vars from the scene
// - overrides pre-existing values for locations, NPCs, and vars
// - removes locations, NPCs (NOT vars) that are not present in the new scene
func (gs *GameState) LoadScene(s *scenario.Scenario, sceneName string) error {
	scene, ok := s.Scenes[sceneName]
	if !ok {
		return fmt.Errorf("scene %s not found in scenario %s", sceneName, s.Name)
	}
	gs.SceneName = sceneName

	// Reset scene turn counter when loading a new scene
	gs.SceneTurnCounter = 0

	// Initialize maps
	if gs.Vars == nil {
		gs.Vars = make(map[string]string)
	}
	if gs.WorldLocations == nil {
		gs.WorldLocations = make(map[string]scenario.Location)
	}
	if gs.NPCs == nil {
		gs.NPCs = make(map[string]scenario.NPC)
	}

	// Copy locations from scene
	if scene.Locations != nil {
		maps.Copy(gs.WorldLocations, scene.Locations)
	}

	// Remove any locations that are not in the global scenario locations,
	// and also not in the current scene locations
	for locName := range gs.WorldLocations {
		_, existsInScenario := s.Locations[locName]
		_, existsInScene := scene.Locations[locName]
		if !existsInScenario && !existsInScene {
			delete(gs.WorldLocations, locName)
		}
	}

	// Copy NPCs from scene
	if scene.NPCs != nil {
		maps.Copy(gs.NPCs, scene.NPCs)
	}

	// Remove any NPCs that are not in the global scenario NPCs,
	// and also not in the current scene NPCs
	for npcName := range gs.NPCs {
		_, existsInScenario := s.NPCs[npcName]
		_, existsInScene := scene.NPCs[npcName]
		if !existsInScenario && !existsInScene {
			delete(gs.NPCs, npcName)
		}
	}

	// Vars from scene
	if scene.Vars != nil {
		maps.Copy(gs.Vars, scene.Vars)
	}

	gs.NormalizeItems()

	return nil
}

// IncrementTurnCounters increments both the turn counter and scene turn counter
// after a successful chat interaction.
func (gs *GameState) IncrementTurnCounters() {
	gs.TurnCounter++
	gs.SceneTurnCounter++
}

// NormalizeItems enforces item singletons by removing duplicate items across:
// - User inventory (highest priority)
// - NPC items (second priority)
// - Location items (lowest priority)
// If an item exists in multiple places, it is removed from the lower priority locations.
func (gs *GameState) NormalizeItems() {
	if gs == nil {
		return
	}

	// Create a set of items in user inventory for fast lookup
	userItems := make(map[string]bool)
	for _, item := range gs.Inventory {
		userItems[item] = true
	}

	// Remove duplicates from NPCs and enforce singletons within NPC collection
	npcItems := make(map[string]bool)
	for npcName, npc := range gs.NPCs {
		var filteredItems []string
		for _, item := range npc.Items {
			// Keep item only if it's not in user inventory and not already claimed by another NPC
			if !userItems[item] && !npcItems[item] {
				filteredItems = append(filteredItems, item)
				npcItems[item] = true
			}
		}
		// Update the NPC in the map
		npc.Items = filteredItems
		gs.NPCs[npcName] = npc
	}

	// Remove duplicates from locations
	for locName, location := range gs.WorldLocations {
		var filteredItems []string
		for _, item := range location.Items {
			// Keep item only if it's not in user inventory or with NPCs
			if !userItems[item] && !npcItems[item] {
				filteredItems = append(filteredItems, item)
			}
		}
		// Update the location in the map
		location.Items = filteredItems
		gs.WorldLocations[locName] = location
	}
}
