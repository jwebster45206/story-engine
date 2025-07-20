package state

import "github.com/jwebster45206/story-engine/pkg/scenario"

// PromptState is a reduced game state for API request to LLM.
type PromptState struct {
	NPCs           map[string]scenario.NPC      `json:"world_npcs,omitempty"`
	WorldLocations map[string]scenario.Location `json:"world_locations,omitempty"` // Current locations in the game world
	Location       string                       `json:"user_location"`
	Inventory      []string                     `json:"user_inventory"`
	// Flags     map[string]bool         `json:"flags"`
}

func ToPromptState(gs *GameState) *PromptState {
	return &PromptState{
		NPCs:           gs.NPCs,
		WorldLocations: gs.WorldLocations,
		Location:       gs.Location,
		Inventory:      gs.Inventory,
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
}
