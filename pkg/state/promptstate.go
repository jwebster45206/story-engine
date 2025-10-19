package state

import (
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// PromptState is a reduced game state for LLM prompts.
// For user-facing prompts, only core world state is included.
// For background processing, Vars are also populated.
type PromptState struct {
	SceneName        string                       `json:"scene_name,omitempty"`         // Current scene name
	NPCs             map[string]actor.NPC         `json:"npcs,omitempty"`               // Map of key NPCs
	WorldLocations   map[string]scenario.Location `json:"locations,omitempty"`          // Current locations in the game world
	Location         string                       `json:"user_location,omitempty"`      // User's current location
	Inventory        []string                     `json:"user_inventory,omitempty"`     // Inventory items
	Vars             map[string]string            `json:"vars,omitempty"`               // Only populated for background processing
	IsEnded          bool                         `json:"is_ended"`                     // true when the game is over
	TurnCounter      int                          `json:"turn_counter,omitempty"`       // Total number of successful chat interactions
	SceneTurnCounter int                          `json:"scene_turn_counter,omitempty"` // Number of successful chat interactions in
}

func ToPromptState(gs *GameState) *PromptState {
	// Filter NPCs: only include those in the same location as user OR marked as important
	filteredNPCs := make(map[string]actor.NPC)
	for name, npc := range gs.NPCs {
		if npc.Location == gs.Location || npc.IsImportant {
			filteredNPCs[name] = npc
		}
	}

	return &PromptState{
		NPCs:           filteredNPCs,
		WorldLocations: filterLocations(gs.WorldLocations, gs.Location),
		Location:       gs.Location,
		Inventory:      gs.Inventory,
		// Vars and counters intentionally excluded for user-facing prompts
	}
}

// filterLocations returns locations that should be included in prompts:
// - The user's current location
// - Locations marked as important
// - Locations adjacent to the current location (accessible via exits)
func filterLocations(worldLocations map[string]scenario.Location, currentLocation string) map[string]scenario.Location {
	filteredLocations := make(map[string]scenario.Location)

	for name, loc := range worldLocations {
		// Include current location or important locations
		if name == currentLocation || loc.IsImportant {
			filteredLocations[name] = loc
		}
	}

	// Also include adjacent locations (accessible via exits from current location)
	if currentLoc, exists := worldLocations[currentLocation]; exists {
		for _, exitLocationKey := range currentLoc.Exits {
			if adjacentLoc, adjacentExists := worldLocations[exitLocationKey]; adjacentExists {
				filteredLocations[exitLocationKey] = adjacentLoc
			}
		}
	}

	return filteredLocations
}

func ToBackgroundPromptState(gs *GameState) *PromptState {
	// Filter NPCs: only include those in the same location as user OR marked as important
	filteredNPCs := make(map[string]actor.NPC)
	for name, npc := range gs.NPCs {
		if npc.Location == gs.Location || npc.IsImportant {
			filteredNPCs[name] = npc
		}
	}

	return &PromptState{
		SceneName:        gs.SceneName,
		NPCs:             filteredNPCs,
		WorldLocations:   filterLocations(gs.WorldLocations, gs.Location),
		Location:         gs.Location,
		Inventory:        gs.Inventory,
		Vars:             gs.Vars,
		IsEnded:          gs.IsEnded,
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
