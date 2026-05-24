package prompts

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// PromptState is a compact, location-scoped view of the game state
// for LLM context. For background processing, Vars are also populated.
type PromptState struct {
	SceneName        string                       `json:"scene_name,omitempty"`         // Current scene name
	NPCs             map[string]actor.NPC         `json:"npcs,omitempty"`               // Map of key NPCs
	Monsters         map[string]actor.Monster     `json:"monsters,omitempty"`           // Monsters at current location
	WorldLocations   map[string]scenario.Location `json:"locations,omitempty"`          // Current locations in the game world
	Location         string                       `json:"user_location,omitempty"`      // User's current location
	Inventory        []string                     `json:"user_inventory,omitempty"`     // Inventory items
	Vars             map[string]string            `json:"vars,omitempty"`               // Only populated for background processing
	IsEnded          bool                         `json:"is_ended"`                     // true when the game is over
	TurnCounter      int                          `json:"turn_counter,omitempty"`       // Total number of successful chat interactions
	SceneTurnCounter int                          `json:"scene_turn_counter,omitempty"` // Number of successful chat interactions in
	JustEntered      bool                         `json:"just_entered,omitempty"`       // true on the first turn after a location change
}

func ToPromptState(gs *state.GameState) *PromptState {
	// Filter NPCs: only include those in the same location as user OR marked as important
	filteredNPCs := make(map[string]actor.NPC)
	for name, npc := range gs.NPCs {
		if npc.Location == gs.Location || npc.IsImportant {
			filteredNPCs[name] = npc
		}
	}

	// Filter Monsters: only include those in the current location
	filteredMonsters := make(map[string]actor.Monster)
	if currentLoc, ok := gs.WorldLocations[gs.Location]; ok {
		for id, monster := range currentLoc.Monsters {
			if monster != nil {
				filteredMonsters[id] = *monster
			}
		}
	}

	return &PromptState{
		NPCs:           filteredNPCs,
		Monsters:       filteredMonsters,
		WorldLocations: filterLocations(gs.WorldLocations, gs.Location),
		Location:       gs.Location,
		Inventory:      gs.Inventory,
		JustEntered:    gs.JustEntered,
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

	// Filter Monsters: only include those in the current location
	filteredMonsters := make(map[string]actor.Monster)
	if currentLoc, ok := gs.WorldLocations[gs.Location]; ok {
		for id, monster := range currentLoc.Monsters {
			if monster != nil {
				filteredMonsters[id] = *monster
			}
		}
	}

	return &PromptState{
		SceneName:        gs.SceneName,
		NPCs:             filteredNPCs,
		Monsters:         filteredMonsters,
		WorldLocations:   filterLocations(gs.WorldLocations, gs.Location),
		Location:         gs.Location,
		Inventory:        gs.Inventory,
		Vars:             gs.Vars,
		IsEnded:          gs.IsEnded,
		TurnCounter:      gs.TurnCounter,
		SceneTurnCounter: gs.SceneTurnCounter,
		JustEntered:      gs.JustEntered,
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
// optimized for LLM comprehension. The output is wrapped in XML-style tags
// so that location boundaries, adjacency, and movement rules are unambiguous
// structural elements rather than ambiguous markdown headers.
//
// Example output:
//
//	<world_state>
//	<just_entered>true</just_entered>
//
//	<current_location>
//	Castle Hallway
//	A long stone corridor.
//
//	Items here: key, map
//	NPCs here: Guard
//	Monsters here:
//	- Giant Rat (AC: 12, HP: 7/7): A massive rat the size of a dog.
//
//	Exits (the ONLY directions reachable this turn):
//	- north -> Great Hall
//	- south -> Dungeon but is blocked (the door is locked)
//	</current_location>
//
//	<adjacent_previews>
//	- north: Great Hall - A grand room with high ceilings.
//	</adjacent_previews>
//
//	<npcs_elsewhere>
//	- Calypso: Sleepy Mermaid
//	</npcs_elsewhere>
//
//	<user_inventory>
//	torch, rope
//	</user_inventory>
//
//	<world_state_rules>
//	- Narrate ONLY current_location. Do not narrate inside adjacent locations.
//	- ...
//	</world_state_rules>
//	</world_state>
func (ps *PromptState) ToString() string {
	var sb strings.Builder

	sb.WriteString("<world_state>\n")
	fmt.Fprintf(&sb, "<just_entered>%t</just_entered>\n\n", ps.JustEntered)

	currentLoc, hasCurrent := ps.WorldLocations[ps.Location]
	ps.writeCurrentLocation(&sb, currentLoc, hasCurrent)
	ps.writeAdjacentPreviews(&sb, currentLoc, hasCurrent)
	ps.writeNPCsElsewhere(&sb)
	ps.writeUserInventory(&sb)
	ps.writeWorldStateRules(&sb, currentLoc, hasCurrent)

	sb.WriteString("</world_state>\n")
	return sb.String()
}

// writeCurrentLocation renders the <current_location> block with name,
// description, items here, NPCs here, monsters here, and exits.
func (ps *PromptState) writeCurrentLocation(sb *strings.Builder, currentLoc scenario.Location, hasCurrent bool) {
	sb.WriteString("<current_location>\n")

	if !hasCurrent {
		fmt.Fprintf(sb, "Unknown location: %s\n", ps.Location)
		sb.WriteString("</current_location>\n")
		return
	}

	sb.WriteString(currentLoc.Name)
	sb.WriteString("\n")
	if currentLoc.Description != "" {
		sb.WriteString(currentLoc.Description)
		sb.WriteString("\n")
	}

	if len(currentLoc.Items) > 0 {
		fmt.Fprintf(sb, "\nItems here: %s\n", strings.Join(currentLoc.Items, ", "))
	}

	presentNames := make([]string, 0)
	for _, npc := range ps.NPCs {
		if npc.Location == ps.Location {
			presentNames = append(presentNames, npc.Name)
		}
	}
	sort.Strings(presentNames)
	if len(presentNames) > 0 {
		fmt.Fprintf(sb, "NPCs here: %s\n", strings.Join(presentNames, ", "))
	}

	if len(ps.Monsters) > 0 {
		monsterIDs := make([]string, 0, len(ps.Monsters))
		for id := range ps.Monsters {
			monsterIDs = append(monsterIDs, id)
		}
		sort.Strings(monsterIDs)
		sb.WriteString("Monsters here:\n")
		for _, id := range monsterIDs {
			m := ps.Monsters[id]
			fmt.Fprintf(sb, "- %s (AC: %d, HP: %d/%d)", m.Name, m.AC, m.HP, m.MaxHP)
			if m.Description != "" {
				fmt.Fprintf(sb, ": %s", m.Description)
			}
			sb.WriteString("\n")
		}
	}

	if len(currentLoc.Exits) > 0 || len(currentLoc.BlockedExits) > 0 {
		sb.WriteString("\nExits (the ONLY directions reachable this turn):\n")
		dirs := collectExitDirections(currentLoc)
		for _, dir := range dirs {
			destKey, hasExit := currentLoc.Exits[dir]
			blockedReason, isBlocked := currentLoc.BlockedExits[dir]

			switch {
			case hasExit && isBlocked:
				destName := ps.locationDisplayName(destKey)
				fmt.Fprintf(sb, "- %s -> %s but is blocked (%s)\n", dir, destName, blockedReason)
			case hasExit:
				destName := ps.locationDisplayName(destKey)
				fmt.Fprintf(sb, "- %s -> %s\n", dir, destName)
			case isBlocked:
				fmt.Fprintf(sb, "- %s is blocked (%s)\n", dir, blockedReason)
			}
		}
	}

	sb.WriteString("</current_location>\n")
}

// writeAdjacentPreviews renders the <adjacent_previews> block: one line per
// adjacent (one-hop) location, using only the Preview field. Locations marked
// IsImportant but not adjacent are listed without a direction prefix.
func (ps *PromptState) writeAdjacentPreviews(sb *strings.Builder, currentLoc scenario.Location, hasCurrent bool) {
	if !hasCurrent {
		return
	}

	dirForLoc := make(map[string][]string)
	for d, destKey := range currentLoc.Exits {
		dirForLoc[destKey] = append(dirForLoc[destKey], d)
	}

	locKeys := make([]string, 0, len(ps.WorldLocations))
	for k := range ps.WorldLocations {
		if k != ps.Location {
			locKeys = append(locKeys, k)
		}
	}
	sort.Strings(locKeys)

	adjacent := make([]string, 0)
	elsewhere := make([]string, 0)
	for _, k := range locKeys {
		loc := ps.WorldLocations[k]
		preview := strings.TrimSpace(loc.Preview)

		if dirs, isAdjacent := dirForLoc[k]; isAdjacent {
			sort.Strings(dirs)
			dirStr := strings.Join(dirs, "/")
			if preview != "" {
				adjacent = append(adjacent, fmt.Sprintf("- %s: %s - %s", dirStr, loc.Name, preview))
			} else {
				adjacent = append(adjacent, fmt.Sprintf("- %s: %s", dirStr, loc.Name))
			}
		} else {
			if preview != "" {
				elsewhere = append(elsewhere, fmt.Sprintf("- %s (elsewhere) - %s", loc.Name, preview))
			} else {
				elsewhere = append(elsewhere, fmt.Sprintf("- %s (elsewhere)", loc.Name))
			}
		}
	}

	if len(adjacent) == 0 && len(elsewhere) == 0 {
		return
	}

	sb.WriteString("\n<adjacent_previews>\n")
	for _, e := range adjacent {
		sb.WriteString(e)
		sb.WriteString("\n")
	}
	for _, e := range elsewhere {
		sb.WriteString(e)
		sb.WriteString("\n")
	}
	sb.WriteString("</adjacent_previews>\n")
}

// writeNPCsElsewhere renders the <npcs_elsewhere> block: name + location only,
// no description. Includes only NPCs whose location differs from the player's.
// (Filtering in ToPromptState already restricts this to important NPCs.)
func (ps *PromptState) writeNPCsElsewhere(sb *strings.Builder) {
	if len(ps.NPCs) == 0 {
		return
	}

	npcKeys := make([]string, 0, len(ps.NPCs))
	for k := range ps.NPCs {
		npcKeys = append(npcKeys, k)
	}
	sort.Strings(npcKeys)

	entries := make([]string, 0)
	for _, k := range npcKeys {
		npc := ps.NPCs[k]
		if npc.Location == ps.Location {
			continue
		}
		locName := ps.locationDisplayName(npc.Location)
		if locName == "" {
			locName = "unknown"
		}
		entries = append(entries, fmt.Sprintf("- %s: %s", npc.Name, locName))
	}

	if len(entries) == 0 {
		return
	}
	sb.WriteString("\n<npcs_elsewhere>\n")
	for _, e := range entries {
		sb.WriteString(e)
		sb.WriteString("\n")
	}
	sb.WriteString("</npcs_elsewhere>\n")
}

// writeUserInventory renders the <user_inventory> block.
func (ps *PromptState) writeUserInventory(sb *strings.Builder) {
	if len(ps.Inventory) == 0 {
		return
	}
	sb.WriteString("\n<user_inventory>\n")
	sb.WriteString(strings.Join(ps.Inventory, ", "))
	sb.WriteString("\n</user_inventory>\n")
}

// writeWorldStateRules renders the <world_state_rules> block, with the
// allowed-destinations enumeration rendered literally from the current
// location's exits.
func (ps *PromptState) writeWorldStateRules(sb *strings.Builder, currentLoc scenario.Location, hasCurrent bool) {
	sb.WriteString("\n<world_state_rules>\n")
	sb.WriteString("- Narrate ONLY current_location. Do not narrate inside adjacent locations.\n")
	sb.WriteString("- Use the description verbatim or paraphrased. You may add ambient sensory detail (smell, temperature, distant sound). Do NOT introduce doors, alcoves, statues, furniture, mechanisms, NPCs, items, or monsters not listed above.\n")
	sb.WriteString("- If just_entered is true, give a brief opening description; otherwise do not re-describe the room - continue the action.\n")

	if hasCurrent && len(currentLoc.Exits) > 0 {
		dirs := make([]string, 0, len(currentLoc.Exits))
		for d := range currentLoc.Exits {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)

		options := make([]string, 0, len(dirs))
		redirects := make([]string, 0, len(dirs))
		for _, d := range dirs {
			destName := ps.locationDisplayName(currentLoc.Exits[d])
			options = append(options, fmt.Sprintf("%s (%s)", d, destName))
			redirects = append(redirects, fmt.Sprintf("%s to %s", d, destName))
		}

		fmt.Fprintf(sb,
			"- Movement: the player may only choose one of: %s. If they try anything else, redirect with: \"You can't go that way. From %s you can go %s.\"\n",
			strings.Join(options, ", "),
			currentLoc.Name,
			joinNatural(redirects),
		)
	}
	sb.WriteString("</world_state_rules>\n")
}

// locationDisplayName resolves a location key to its display name, falling
// back to the key itself if the location is not in WorldLocations.
func (ps *PromptState) locationDisplayName(key string) string {
	if key == "" {
		return ""
	}
	if loc, ok := ps.WorldLocations[key]; ok && loc.Name != "" {
		return loc.Name
	}
	return key
}

// collectExitDirections returns a sorted, de-duplicated list of all
// directions referenced by either Exits or BlockedExits on the given location.
func collectExitDirections(loc scenario.Location) []string {
	seen := make(map[string]struct{}, len(loc.Exits)+len(loc.BlockedExits))
	for d := range loc.Exits {
		seen[d] = struct{}{}
	}
	for d := range loc.BlockedExits {
		seen[d] = struct{}{}
	}
	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// joinNatural joins items with commas and a final " or ":
//
//	[]            -> ""
//	[a]           -> "a"
//	[a, b]        -> "a or b"
//	[a, b, c]     -> "a, b, or c"
func joinNatural(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", or " + items[len(items)-1]
	}
}
