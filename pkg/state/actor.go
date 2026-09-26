package state

import (
	"maps"
	"slices"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/character"
)

// FindActor resolves name to a live mechanical block and its display name.
// The PC matches by display name. The literal "PC" matches only when that is
// PCName. NPCs match by map key or name at the current location. Monsters
// match by instance id, Monster.ID, or name in the current location; a shared
// name resolves to the lowest sorted instance id.
//
// ok is true only when the match IsActor, so a narrative NPC is not a target.
func (gs *GameState) FindActor(name string) (*character.Stats, string, bool) {
	name = strings.TrimSpace(name)
	if gs == nil || name == "" {
		return nil, "", false
	}
	if stats, display, ok := gs.findPCActor(name); ok {
		return stats, display, true
	}
	if stats, display, ok := gs.findNPCActor(name); ok {
		return stats, display, true
	}
	return gs.findMonsterActor(name)
}

func (gs *GameState) findPCActor(name string) (*character.Stats, string, bool) {
	if gs.PC == nil {
		return nil, "", false
	}
	pcName := gs.PCName()
	if !strings.EqualFold(name, pcName) {
		return nil, "", false
	}
	return actorStats(&gs.PC.Stats, pcName, pcName)
}

func (gs *GameState) findNPCActor(name string) (*character.Stats, string, bool) {
	keys := slices.Sorted(maps.Keys(gs.NPCs))
	for _, key := range keys {
		npc := gs.NPCs[key]
		if npc == nil || npc.Location != gs.Location {
			continue
		}
		if strings.EqualFold(key, name) {
			return actorStats(&npc.Stats, npc.Name, key)
		}
	}
	for _, key := range keys {
		npc := gs.NPCs[key]
		if npc == nil || npc.Location != gs.Location {
			continue
		}
		if strings.EqualFold(npc.Name, name) {
			return actorStats(&npc.Stats, npc.Name, key)
		}
	}
	return nil, "", false
}

func (gs *GameState) findMonsterActor(name string) (*character.Stats, string, bool) {
	loc, ok := gs.WorldLocations[gs.Location]
	if !ok {
		return nil, "", false
	}
	ids := slices.Sorted(maps.Keys(loc.Monsters))
	for _, id := range ids {
		m := loc.Monsters[id]
		if m == nil {
			continue
		}
		if strings.EqualFold(id, name) || strings.EqualFold(m.ID, name) {
			return actorStats(&m.Stats, m.Name, id)
		}
	}
	for _, id := range ids {
		m := loc.Monsters[id]
		if m == nil {
			continue
		}
		if strings.EqualFold(m.Name, name) {
			return actorStats(&m.Stats, m.Name, id)
		}
	}
	return nil, "", false
}

func actorStats(s *character.Stats, display, fallback string) (*character.Stats, string, bool) {
	if s == nil || !s.IsActor() {
		return nil, "", false
	}
	if strings.TrimSpace(display) == "" {
		display = fallback
	}
	return s, display, true
}
