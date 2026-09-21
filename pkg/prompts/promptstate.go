package prompts

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// PromptState is a compact, location-scoped view of the game state
// for LLM context. For background processing, Vars are also populated.
type PromptState struct {
	SceneName        string                       `json:"scene_name,omitempty"`         // Current scene name
	NPCs             map[string]character.NPC     `json:"npcs,omitempty"`               // Map of key NPCs
	Monsters         map[string]character.Monster `json:"monsters,omitempty"`           // Monsters at current location
	WorldLocations   map[string]scenario.Location `json:"locations,omitempty"`          // Current locations in the game world
	Location         string                       `json:"user_location,omitempty"`      // User's current location
	Inventory        []string                     `json:"user_inventory,omitempty"`     // Inventory items
	Vars             map[string]string            `json:"vars,omitempty"`               // Only populated for background processing
	IsEnded          bool                         `json:"is_ended"`                     // true when the game is over
	TurnCounter      int                          `json:"turn_counter,omitempty"`       // Total number of successful chat interactions
	SceneTurnCounter int                          `json:"scene_turn_counter,omitempty"` // Number of successful chat interactions in
	Rules            state.RulesMode              `json:"-"`                            // Narrator ruleset; not sent to reducer JSON
	FocusedActors    []string                     `json:"-"`                            // Non-PC ruling subject/object names; not sent to reducer JSON
}

func ToPromptState(gs *state.GameState) *PromptState {
	return toPromptStateAt(gs, gs.Location)
}

// toPromptStateAt builds a PromptState as if the player were already at viewLoc.
// GameState is not mutated. When viewLoc is a movement destination, NPCs that
// follow the PC are treated as present there so companions do not vanish on
// the arrival prompt; Apply still syncs their locations after narration.
func toPromptStateAt(gs *state.GameState, viewLoc string) *PromptState {
	filteredNPCs := make(map[string]character.NPC)
	for name, npc := range gs.NPCs {
		if viewLoc != gs.Location && strings.EqualFold(npc.Following, "pc") {
			clone := npc
			clone.Location = viewLoc
			filteredNPCs[name] = clone
			continue
		}
		if npc.Location == viewLoc || npc.IsImportant {
			filteredNPCs[name] = npc
		}
	}

	filteredMonsters := make(map[string]character.Monster)
	if currentLoc, ok := gs.WorldLocations[viewLoc]; ok {
		for id, monster := range currentLoc.Monsters {
			if monster != nil {
				filteredMonsters[id] = *monster
			}
		}
	}

	return &PromptState{
		NPCs:           filteredNPCs,
		Monsters:       filteredMonsters,
		WorldLocations: filterLocations(gs.WorldLocations, viewLoc),
		Location:       viewLoc,
		Inventory:      gs.Inventory,
		Rules:          gs.Rules,
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
	filteredNPCs := make(map[string]character.NPC)
	for name, npc := range gs.NPCs {
		if npc.Location == gs.Location || npc.IsImportant {
			filteredNPCs[name] = npc
		}
	}

	// Filter Monsters: only include those in the current location
	filteredMonsters := make(map[string]character.Monster)
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
// optimized for LLM comprehension. A nil ruling (and any non-movement ruling)
// renders the full slim state. A resolved movement destination is applied by
// building PromptState at that location before calling ToString; this slice
// does not further trim sections by scope.
func (ps *PromptState) ToString(ruling *chat.Ruling) string {
	var sb strings.Builder

	sb.WriteString("<world_state>\n")

	currentLoc, hasCurrent := ps.WorldLocations[ps.Location]
	ps.writeCurrentLocation(&sb, currentLoc, hasCurrent, true)
	ps.writeAdjacentPreviews(&sb, currentLoc, hasCurrent)
	ps.writeNPCsElsewhere(&sb)
	ps.writeUserInventory(&sb)
	ps.writeWorldStateRules(&sb)

	sb.WriteString("</world_state>\n")
	return sb.String()
}

func (ps *PromptState) ToSlimString() string {
	var sb strings.Builder

	sb.WriteString("<world_state>\n")

	currentLoc, hasCurrent := ps.WorldLocations[ps.Location]
	ps.writeCurrentLocation(&sb, currentLoc, hasCurrent, false)
	ps.writeAdjacentPreviews(&sb, currentLoc, hasCurrent)
	ps.writeNPCsElsewhere(&sb)
	ps.writeUserInventory(&sb)

	sb.WriteString("</world_state>\n")
	return sb.String()
}

// writeCurrentLocation renders the <current_location> block with name,
// description, items here, NPCs here, monsters here, and exits.
// monsterFlavor includes AC/HP and description on monster lines.
func (ps *PromptState) writeCurrentLocation(sb *strings.Builder, currentLoc scenario.Location, hasCurrent bool, monsterFlavor bool) {
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

	if present := presentNPCs(ps.NPCs, ps.Location); len(present) > 0 {
		if !anyNPCFocused(present, ps.FocusedActors) {
			names := make([]string, len(present))
			for i, npc := range present {
				names[i] = npc.Name
			}
			fmt.Fprintf(sb, "NPCs here: %s\n", strings.Join(names, ", "))
		} else {
			sb.WriteString("NPCs here:\n")
			for _, npc := range present {
				if npcFocused(npc, ps.FocusedActors) && npc.Description != "" {
					fmt.Fprintf(sb, "- %s: %s\n", npc.Name, npc.Description)
				} else {
					fmt.Fprintf(sb, "- %s\n", npc.Name)
				}
			}
		}
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
			if monsterFlavor {
				fmt.Fprintf(sb, "- %s (AC: %d, HP: %d/%d)", m.Name, m.AC, m.HP, m.MaxHP)
				if m.Description != "" {
					fmt.Fprintf(sb, ": %s", m.Description)
				}
				sb.WriteString("\n")
			} else {
				fmt.Fprintf(sb, "- %s\n", m.Name)
			}
		}
	}

	if len(currentLoc.Exits) > 0 || len(currentLoc.BlockedExits) > 0 {
		sb.WriteString("\nExits:\n")
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

// writeWorldStateRules renders the <world_state_rules> block from the
// active ruleset: storytelling scope (current_location)
func (ps *PromptState) writeWorldStateRules(sb *strings.Builder) {
	rs := getRuleSet(ps.Rules)
	if len(rs.WorldStateRules) == 0 {
		return
	}
	sb.WriteString("\n<world_state_rules>\n")
	for _, r := range rs.WorldStateRules {
		fmt.Fprintf(sb, "- %s\n", r)
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

func presentNPCs(npcs map[string]character.NPC, location string) []character.NPC {
	present := make([]character.NPC, 0)
	for _, npc := range npcs {
		if npc.Location == location {
			present = append(present, npc)
		}
	}
	sort.Slice(present, func(i, j int) bool { return present[i].Name < present[j].Name })
	return present
}

func npcFocused(npc character.NPC, focused []string) bool {
	for _, name := range focused {
		if strings.EqualFold(name, npc.Name) {
			return true
		}
	}
	return false
}

func anyNPCFocused(npcs []character.NPC, focused []string) bool {
	for _, npc := range npcs {
		if npcFocused(npc, focused) {
			return true
		}
	}
	return false
}

type resolvedLocation struct {
	Key string
	Loc scenario.Location
}

func lookupLocation(world map[string]scenario.Location, name string) (string, scenario.Location, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", scenario.Location{}, false
	}
	if loc, ok := world[name]; ok {
		return name, loc, true
	}
	for key, loc := range world {
		if strings.EqualFold(key, name) || strings.EqualFold(loc.Name, name) {
			return key, loc, true
		}
	}
	return "", scenario.Location{}, false
}

func adjacentLocationKeys(world map[string]scenario.Location, current string) map[string]struct{} {
	out := make(map[string]struct{})
	cur, ok := world[current]
	if !ok {
		return out
	}
	for _, dest := range cur.Exits {
		out[dest] = struct{}{}
	}
	return out
}

func resolveMovementDestinations(gs *state.GameState, names []string) []resolvedLocation {
	if gs == nil || len(names) == 0 {
		return nil
	}
	adjacent := adjacentLocationKeys(gs.WorldLocations, gs.Location)
	out := make([]resolvedLocation, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		key, loc, ok := lookupLocation(gs.WorldLocations, name)
		if !ok || key == gs.Location {
			continue
		}
		if _, reachable := adjacent[key]; !reachable {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, resolvedLocation{Key: key, Loc: loc})
	}
	return out
}
