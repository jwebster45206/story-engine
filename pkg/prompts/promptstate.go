package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
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

func ToPromptState(gs *state.GameState) *PromptState {
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

func ToBackgroundPromptState(gs *state.GameState) *PromptState {
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
func ApplyPromptStateToGameState(ps *PromptState, gs *state.GameState) {
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

// ToString converts the PromptState into a human-readable string format
// optimized for LLM comprehension. Focuses on clear descriptions over IDs.
//
// Example output:
// CURRENT LOCATION:
// Castle Hallway: A long stone corridor.
// Items located here: key, map
//
// Exits:
// - north leads to Great Hall
// - south leads to Dungeon
// - south is blocked (the door is locked)
//
// NEARBY LOCATIONS:
// Great Hall: A grand room with high ceilings and ornate decorations.
//
// NPCs:
// Guard (neutral): A stern-looking guard in armor.
// Items: sword, shield
//
// USER'S INVENTORY:
// torch, rope
func (ps *PromptState) ToString() string {
	var sb strings.Builder

	// Current Location
	sb.WriteString("CURRENT LOCATION:\n")
	if currentLoc, ok := ps.WorldLocations[ps.Location]; ok {
		sb.WriteString(currentLoc.Name)
		if currentLoc.Description != "" {
			sb.WriteString(fmt.Sprintf(": %s", currentLoc.Description))
		}
		sb.WriteString("\n")
		if len(currentLoc.Items) > 0 {
			sb.WriteString("Items located here: ")
			sb.WriteString(strings.Join(currentLoc.Items, ", "))
			sb.WriteString("\n")
		}

		if len(currentLoc.Exits) > 0 || len(currentLoc.BlockedExits) > 0 {
			sb.WriteString("\nExits:\n")
			for direction, locationID := range currentLoc.Exits {
				if destLoc, ok := currentLoc.BlockedExits[direction]; ok {
					sb.WriteString(fmt.Sprintf("- %s is blocked (%s)\n", direction, destLoc))
					continue
				}
				if destLoc, ok := ps.WorldLocations[locationID]; ok {
					sb.WriteString(fmt.Sprintf("- %s leads to %s\n", direction, destLoc.Name))
					continue
				}
				// an undefined locationID is skipped
			}
		}
	} else {
		sb.WriteString(fmt.Sprintf("Unknown location: %s\n", ps.Location))
	}

	// Other Locations (adjacent or important)
	otherLocations := make([]scenario.Location, 0)
	for id, loc := range ps.WorldLocations {
		if id != ps.Location {
			otherLocations = append(otherLocations, loc)
		}
	}
	if len(otherLocations) > 0 {
		sb.WriteString("\nNEARBY LOCATIONS:")
		for _, loc := range otherLocations {
			sb.WriteString(fmt.Sprintf("\n%s", loc.Name))
			if loc.Description != "" {
				sb.WriteString(fmt.Sprintf(": %s", loc.Description))
			}
			sb.WriteString("\n")
		}
	}

	// NPCs
	if len(ps.NPCs) > 0 {
		sb.WriteString("\nNPCs:")
		for _, npc := range ps.NPCs {
			sb.WriteString(fmt.Sprintf("\n%s", npc.Name))
			if npc.Disposition != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", npc.Disposition))
			}

			if npc.Description != "" {
				sb.WriteString(fmt.Sprintf(": %s", npc.Description))
			}

			if len(npc.Items) > 0 {
				sb.WriteString(fmt.Sprintf("; Items: %s\n", strings.Join(npc.Items, ", ")))
			}
			sb.WriteString("\n")
		}
	}

	// User Inventory
	if len(ps.Inventory) > 0 {
		sb.WriteString("\nUSER'S INVENTORY: \n")
		sb.WriteString(strings.Join(ps.Inventory, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}
