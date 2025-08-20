package state

import "github.com/jwebster45206/story-engine/pkg/scenario"

// PromptState is a reduced game state for LLM prompts.
// For user-facing prompts, only core world state is included.
// For background processing, Vars are also populated.
type PromptState struct {
	NPCs           map[string]scenario.NPC      `json:"world_npcs,omitempty"`
	WorldLocations map[string]scenario.Location `json:"world_locations,omitempty"` // Current locations in the game world
	Location       string                       `json:"user_location"`
	Inventory      []string                     `json:"user_inventory"`
	Vars           map[string]string            `json:"vars,omitempty"` // Only populated for background processing
}

func ToPromptState(gs *GameState) *PromptState {
	return &PromptState{
		NPCs:           gs.NPCs,
		WorldLocations: gs.WorldLocations,
		Location:       gs.Location,
		Inventory:      gs.Inventory,
		// Vars intentionally excluded for user-facing prompts
	}
}

func ToBackgroundPromptState(gs *GameState) *PromptState {
	return &PromptState{
		NPCs:           gs.NPCs,
		WorldLocations: gs.WorldLocations,
		Location:       gs.Location,
		Inventory:      gs.Inventory,
		Vars:           gs.Vars,
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