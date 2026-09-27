package state

import (
	"maps"
	"slices"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/character"
)

// Combatant is a live actor. Stats points into game state.
// PC or NPC is the container that owns those stats.
// MonsterID is the location map key when the actor is a monster.
type Combatant struct {
	Stats     *character.Stats
	Display   string
	PC        *character.PC
	NPC       *character.NPC
	MonsterID string
}

// FindActor resolves name to a live mechanical block and its display name.
// The PC matches by display name. The literal "PC" matches only when that is
// PCName. NPCs match by map key or name at the current location. Monsters
// match by instance id, Monster.ID, or name in the current location; a shared
// name resolves to the lowest sorted instance id.
//
// ok is true only when the match IsActor, so a narrative NPC is not a target.
// A knocked-out NPC is not a target either.
func (gs *GameState) FindActor(name string) (*character.Stats, string, bool) {
	c, ok := gs.FindCombatant(name)
	if !ok {
		return nil, "", false
	}
	return c.Stats, c.Display, true
}

// FindCombatant resolves name the same way as FindActor and also reports
// which container the stats belong to, so a knockout can despawn or flag it.
func (gs *GameState) FindCombatant(name string) (Combatant, bool) {
	name = strings.TrimSpace(name)
	if gs == nil || name == "" {
		return Combatant{}, false
	}
	if c, ok := gs.findPCCombatant(name); ok {
		return c, true
	}
	if c, ok := gs.findNPCCombatant(name); ok {
		return c, true
	}
	return gs.findMonsterCombatant(name)
}

// KnockOut applies the 0 HP outcome for the combatant who just fell.
// Monsters drop loot and leave. NPCs stay, marked defeated. The PC ends the game.
func (gs *GameState) KnockOut(c Combatant) {
	if gs == nil || c.Stats == nil || c.Stats.HP > 0 {
		return
	}
	switch {
	case c.PC != nil:
		gs.IsEnded = true
	case c.NPC != nil:
		c.NPC.IsDefeated = true
	case c.MonsterID != "":
		gs.DespawnMonster(c.MonsterID)
	}
}

func (gs *GameState) findPCCombatant(name string) (Combatant, bool) {
	if gs.PC == nil {
		return Combatant{}, false
	}
	pcName := gs.PCName()
	if !strings.EqualFold(name, pcName) {
		return Combatant{}, false
	}
	stats, display, ok := actorStats(&gs.PC.Stats, pcName, pcName)
	if !ok {
		return Combatant{}, false
	}
	return Combatant{Stats: stats, Display: display, PC: gs.PC}, true
}

func (gs *GameState) findNPCCombatant(name string) (Combatant, bool) {
	keys := slices.Sorted(maps.Keys(gs.NPCs))
	for _, key := range keys {
		if c, ok := gs.npcCombatant(key, name, true); ok {
			return c, true
		}
	}
	for _, key := range keys {
		if c, ok := gs.npcCombatant(key, name, false); ok {
			return c, true
		}
	}
	return Combatant{}, false
}

func (gs *GameState) npcCombatant(key, name string, matchKey bool) (Combatant, bool) {
	npc := gs.NPCs[key]
	if npc == nil || npc.IsDefeated || npc.Location != gs.Location {
		return Combatant{}, false
	}
	match := strings.EqualFold(npc.Name, name)
	if matchKey {
		match = strings.EqualFold(key, name)
	}
	if !match {
		return Combatant{}, false
	}
	stats, display, ok := actorStats(&npc.Stats, npc.Name, key)
	if !ok {
		return Combatant{}, false
	}
	return Combatant{Stats: stats, Display: display, NPC: npc}, true
}

func (gs *GameState) findMonsterCombatant(name string) (Combatant, bool) {
	loc, ok := gs.WorldLocations[gs.Location]
	if !ok {
		return Combatant{}, false
	}
	ids := slices.Sorted(maps.Keys(loc.Monsters))
	for _, id := range ids {
		if c, ok := monsterCombatant(loc.Monsters[id], id, name, true); ok {
			return c, true
		}
	}
	for _, id := range ids {
		if c, ok := monsterCombatant(loc.Monsters[id], id, name, false); ok {
			return c, true
		}
	}
	return Combatant{}, false
}

func monsterCombatant(m *character.Monster, id, name string, matchID bool) (Combatant, bool) {
	if m == nil {
		return Combatant{}, false
	}
	match := strings.EqualFold(m.Name, name)
	if matchID {
		match = strings.EqualFold(id, name) || strings.EqualFold(m.ID, name)
	}
	if !match {
		return Combatant{}, false
	}
	stats, display, ok := actorStats(&m.Stats, m.Name, id)
	if !ok {
		return Combatant{}, false
	}
	return Combatant{Stats: stats, Display: display, MonsterID: id}, true
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
