package state

import "github.com/jwebster45206/story-engine/pkg/scenario"

// PromptState is a reduced game state for LLM prompts.
// For user-facing prompts, Vars and ContingencyPrompts are excluded.
// For background processing, all fields are populated.
type PromptState struct {
	NPCs               map[string]scenario.NPC      `json:"world_npcs,omitempty"`
	WorldLocations     map[string]scenario.Location `json:"world_locations,omitempty"` // Current locations in the game world
	Location           string                       `json:"user_location"`
	Inventory          []string                     `json:"user_inventory"`
	Vars               map[string]string            `json:"vars,omitempty"`               // Only populated for background processing
	ContingencyPrompts []string                     `json:"contingency_prompts,omitempty"` // Only populated for background processing
}

func ToPromptState(gs *GameState) *PromptState {
	return &PromptState{
		NPCs:           gs.NPCs,
		WorldLocations: gs.WorldLocations,
		Location:       gs.Location,
		Inventory:      gs.Inventory,
		// Vars and ContingencyPrompts intentionally excluded for user-facing prompts
	}
}

func ToBackgroundPromptState(gs *GameState) *PromptState {
	return &PromptState{
		NPCs:               gs.NPCs,
		WorldLocations:     gs.WorldLocations,
		Location:           gs.Location,
		Inventory:          gs.Inventory,
		Vars:               gs.Vars,
		ContingencyPrompts: gs.ContingencyPrompts,
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
	// ContingencyPrompts are not copied as they're not stateful
}