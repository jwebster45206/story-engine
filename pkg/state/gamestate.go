package state

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// RulesMode selects the player-latitude ruleset for a game session.
type RulesMode string

const (
	RulesStrict  RulesMode = "strict"
	RulesRelaxed RulesMode = "relaxed"
)

// DefaultTemperature is the LLM sampling temperature used when none is set on the GameState.
const DefaultTemperature = 0.6

// DefaultPCID is the fallback PC id when neither the request nor the scenario specifies one.
const DefaultPCID = "classic"

// GameState stores the current state of the game
type GameState struct {
	ID                 uuid.UUID                    `json:"id"`                           // Unique ID per session
	Provider           string                       `json:"provider,omitempty"`           // Named provider from config (vendor+model)
	ModelName          string                       `json:"model_name,omitempty"`         // Server-stamped from the provider's model; ignored on input
	Scenario           string                       `json:"scenario,omitempty"`           // Filename of the scenario being played. Ex: "foo_scenario.json"
	SceneName          string                       `json:"scene_name,omitempty"`         // Current scene name in the scenario, if applicable
	Rules              RulesMode                    `json:"rules,omitempty"`              // "strict" (default) or "relaxed"
	Temperature        float64                      `json:"temperature,omitempty"`        // LLM sampling temperature (0.0-1.0)
	Narrator           *scenario.Narrator           `json:"narrator,omitempty"`           // Embedded narrator for this game session (loaded once at creation)
	PC                 *actor.PC                    `json:"pc,omitempty"`                 // Player Character for this game session
	NPCs               map[string]actor.NPC         `json:"npcs,omitempty"`               // All NPCs in the game world
	WorldLocations     map[string]scenario.Location `json:"locations,omitempty"`          // Current locations in the game world
	Location           string                       `json:"user_location,omitempty"`      // Current location in the game world
	Inventory          []string                     `json:"user_inventory,omitempty"`     // User's inventory items
	ChatHistory        []chat.ChatMessage           `json:"chat_history,omitempty"`       // Conversation history
	TurnCounter        int                          `json:"turn_counter"`                 // Total number of successful chat interactions
	SceneTurnCounter   int                          `json:"scene_turn_counter"`           // Number of successful chat interactions in current scene
	Vars               map[string]string            `json:"vars,omitempty"`               // Game variables (e.g. flags, counters)
	FiredStoryEvents   []string                     `json:"fired_story_events,omitempty"` // IDs of story events that have already fired (never fire twice)
	IsEnded            bool                         `json:"is_ended"`                     // true when the game is over
	ContingencyPrompts []string                     `json:"contingency_prompts,omitempty"`
	CreatedAt          time.Time                    `json:"created_at"`
	UpdatedAt          time.Time                    `json:"updated_at"`

	// JustEntered is true on the first turn after a location change.
	// Transient: set by the delta worker when Apply() changes Location,
	// cleared on the next Apply() that does not change Location. Not
	// persisted (json:"-") so storage churn stays bounded; the narrator
	// sees the signal once and then it resets.
	JustEntered bool `json:"-"`
}

func NewGameState(scenarioFileName string, narrator *scenario.Narrator, provider string, modelName string) *GameState {
	return &GameState{
		ID:                 uuid.New(),
		Provider:           provider,
		ModelName:          modelName,
		Scenario:           scenarioFileName,
		Rules:              RulesStrict,
		Temperature:        DefaultTemperature,
		Narrator:           narrator, // Embed full narrator object
		ChatHistory:        make([]chat.ChatMessage, 0),
		TurnCounter:        0,
		SceneTurnCounter:   0,
		Vars:               make(map[string]string),
		FiredStoryEvents:   make([]string, 0),
		ContingencyPrompts: make([]string, 0),
		NPCs:               make(map[string]actor.NPC),
		WorldLocations:     make(map[string]scenario.Location),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
}

// Validate checks create-request fields before Normalize applies defaults.
// Empty Rules and zero Temperature are allowed (Normalize fills them in).
func (gs *GameState) Validate() error {
	if gs.Scenario == "" {
		return fmt.Errorf("scenario field is required")
	}
	if gs.Rules != "" && gs.Rules != RulesStrict && gs.Rules != RulesRelaxed {
		return fmt.Errorf("rules must be %q or %q", RulesStrict, RulesRelaxed)
	}
	if gs.Temperature < 0.0 || gs.Temperature > 1.0 {
		return fmt.Errorf("temperature must be between 0.0 and 1.0")
	}
	return nil
}

// Normalize applies casing and defaults for create-request fields.
// Call after Validate. Rules and Temperature defaults are applied here;
// Scenario gets snake_case + .json; Narrator.ID and PC.Spec.ID get snake_case.
func (gs *GameState) Normalize() {
	gs.Scenario = normalizeID(gs.Scenario)
	gs.Scenario = ensureJSONExtension(gs.Scenario)

	if gs.Narrator != nil {
		gs.Narrator.ID = stripJSONExtension(normalizeID(gs.Narrator.ID))
	}
	if gs.PC != nil && gs.PC.Spec != nil {
		gs.PC.Spec.ID = stripJSONExtension(normalizeID(gs.PC.Spec.ID))
	}

	if gs.Rules == "" {
		gs.Rules = RulesStrict
	}
	if gs.Temperature == 0 {
		gs.Temperature = DefaultTemperature
	}
}

// normalizeID converts a string to lowercase snake_case for consistent IDs.
// It handles spaces, hyphens, dots, and camelCase/PascalCase.
func normalizeID(s string) string {
	if s == "" {
		return ""
	}

	var out strings.Builder
	prevUnderscore := false
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			r = r + ('a' - 'A')
		}
		switch {
		case r == '.':
			out.WriteRune('.')
			prevUnderscore = false

		case r == ' ' || r == '-' || r == '_':
			if !prevUnderscore && i > 0 {
				out.WriteRune('_')
				prevUnderscore = true
			}

		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			out.WriteRune(r)
			prevUnderscore = false

		default:
			// Ignore other characters
		}
	}
	return out.String()
}

// ensureJSONExtension adds .json extension if not present
func ensureJSONExtension(s string) string {
	if s == "" {
		return ""
	}
	if !strings.HasSuffix(s, ".json") {
		return s + ".json"
	}
	return s
}

// stripJSONExtension removes .json extension if present
func stripJSONExtension(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimSuffix(s, ".json")
}

// GetContingencyPrompts returns all applicable contingency prompts for the current game state
// Filters prompts based on their conditional requirements
func (gs *GameState) GetContingencyPrompts(s *scenario.Scenario) []string {
	if gs == nil || s == nil {
		return nil
	}

	var prompts []string

	// Filter scenario-level contingency prompts based on conditions
	scenarioPrompts := scenario.FilterContingencyPrompts(s.ContingencyPrompts, gs)
	prompts = append(prompts, scenarioPrompts...)

	// Filter PC-level contingency prompts based on conditions
	if gs.PC != nil && gs.PC.Spec != nil {
		pcPrompts := scenario.FilterContingencyPrompts(gs.PC.Spec.ContingencyPrompts, gs)
		prompts = append(prompts, pcPrompts...)
	}

	// Add custom gamestate-level prompts (already stored as strings, always shown)
	prompts = append(prompts, gs.ContingencyPrompts...)

	// Filter scene-level contingency prompts if in a scene
	if gs.SceneName != "" {
		if scene, ok := s.Scenes[gs.SceneName]; ok {
			scenePrompts := scenario.FilterContingencyPrompts(scene.ContingencyPrompts, gs)
			prompts = append(prompts, scenePrompts...)
		}
	}

	// NPC-level contingency prompts
	for _, npc := range gs.NPCs {
		// Only include prompts for NPCs at the player's current location
		if npc.Location != gs.Location {
			continue
		}
		prompts = append(prompts, scenario.FilterContingencyPrompts(npc.ContingencyPrompts, gs)...)
	}

	// Location-level contingency prompts
	if location, ok := gs.WorldLocations[gs.Location]; ok {
		prompts = append(prompts, scenario.FilterContingencyPrompts(location.ContingencyPrompts, gs)...)
	}

	return prompts
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
		gs.NPCs = make(map[string]actor.NPC)
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

func (gs *GameState) GetSceneName() string {
	return gs.SceneName
}

func (gs *GameState) GetVars() map[string]string {
	return gs.Vars
}

func (gs *GameState) GetSceneTurnCounter() int {
	return gs.SceneTurnCounter
}

func (gs *GameState) GetTurnCounter() int {
	return gs.TurnCounter
}

func (gs *GameState) GetUserLocation() string {
	return gs.Location
}

// SpawnMonster creates a new monster instance from a template.
func (gs *GameState) SpawnMonster(template *actor.Monster, monsterDef *actor.Monster) *actor.Monster {
	if monsterDef == nil || template == nil {
		return nil
	}

	location := monsterDef.Location
	loc, ok := gs.WorldLocations[location]
	if !ok {
		return nil // Location doesn't exist
	}

	if loc.Monsters == nil {
		loc.Monsters = make(map[string]*actor.Monster)
	}

	// Create monster from template with scenario overrides
	m := actor.NewMonster(template, monsterDef)
	if m == nil {
		return nil
	}

	loc.Monsters[monsterDef.ID] = m
	gs.WorldLocations[location] = loc
	return m
}

// DespawnMonster removes a monster instance from the game state.
// If the monster has DropItemsOnDefeat enabled, its items are transferred to its location.
func (gs *GameState) DespawnMonster(instanceID string) {
	for locName, loc := range gs.WorldLocations {
		if m, ok := loc.Monsters[instanceID]; ok {
			// Drop items to location if enabled
			if m.DropItemsOnDefeat && len(m.Items) > 0 {
				loc.Items = append(loc.Items, m.Items...)
			}

			// Remove from location's monster map
			delete(loc.Monsters, instanceID)
			gs.WorldLocations[locName] = loc
			return
		}
	}
}

// EvaluateDefeats checks all active monsters and despawns any that are defeated (HP <= 0).
// This should be called after any action that could change monster HP.
// func (gs *GameState) EvaluateDefeats() {
// 	for _, loc := range gs.WorldLocations {
// 		for id, m := range loc.Monsters {
// 			if m.IsDefeated() {
// 				gs.DespawnMonster(id)
// 			}
// 		}
// 	}
// }
