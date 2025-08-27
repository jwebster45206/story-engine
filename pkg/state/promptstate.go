package state

import "github.com/jwebster45206/story-engine/pkg/scenario"

// PromptState is a reduced game state for LLM prompts.
// For user-facing prompts, only core world state is included.
// For background processing, Vars are also populated.
type PromptState struct {
	SceneName        string                       `json:"scene_name,omitempty"`         // Current scene name
	NPCs             map[string]scenario.NPC      `json:"npcs,omitempty"`               // Map of key NPCs
	WorldLocations   map[string]scenario.Location `json:"locations,omitempty"`          // Current locations in the game world
	Location         string                       `json:"user_location,omitempty"`      // User's current location
	Inventory        []string                     `json:"user_inventory,omitempty"`     // Inventory items
	Vars             map[string]string            `json:"vars,omitempty"`               // Only populated for background processing
	TurnCounter      int                          `json:"turn_counter,omitempty"`       // Total number of successful chat interactions
	SceneTurnCounter int                          `json:"scene_turn_counter,omitempty"` // Number of successful chat interactions in
}

func ToPromptState(gs *GameState) *PromptState {
	return &PromptState{
		NPCs:           gs.NPCs,
		WorldLocations: gs.WorldLocations,
		Location:       gs.Location,
		Inventory:      gs.Inventory,
		// Vars and counters intentionally excluded for user-facing prompts
	}
}

func ToBackgroundPromptState(gs *GameState) *PromptState {
	return &PromptState{
		SceneName:        gs.SceneName,
		NPCs:             gs.NPCs,
		WorldLocations:   gs.WorldLocations,
		Location:         gs.Location,
		Inventory:        gs.Inventory,
		Vars:             gs.Vars,
		TurnCounter:      gs.TurnCounter,
		SceneTurnCounter: gs.SceneTurnCounter,
		// ContingencyPrompts are handled as separate system messages, not JSON data
	}
}

// ApplyPromptStateToGameState copies fields from a PromptState to a GameState.
func ApplyPromptStateToGameState(ps *PromptState, gs *GameState) {
	if ps == nil || gs == nil {
		return
	}
	gs.Location = ps.Location
	gs.Inventory = ps.Inventory
	gs.NPCs = ps.NPCs
	gs.WorldLocations = ps.WorldLocations
	if ps.Vars != nil {
		gs.Vars = ps.Vars
	}
	// ContingencyPrompts are never copied as they're handled separately
}
