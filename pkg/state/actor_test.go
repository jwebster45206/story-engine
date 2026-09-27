package state

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestFindCombatant(t *testing.T) {
	ratA := &character.Monster{ID: "rat_a", Name: "Giant Rat", HP: 4, AC: 8}
	ratB := &character.Monster{ID: "rat_b", Name: "Giant Rat", HP: 9, AC: 12}
	wolf := &character.Monster{ID: "wolf_1", Name: "Wolf", HP: 11}
	gs := &GameState{
		Location: "tavern",
		PC:       &character.PC{Name: "Felix", HP: 12, AC: 14},
		NPCs: map[string]*character.NPC{
			"guard":     {Name: "Guard Captain", Location: "tavern", HP: 10, AC: 16},
			"pip":       {Name: "Pip Upton", Location: "cellar", HP: 5},
			"bartender": {Name: "Bartender", Location: "tavern"},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Monsters: map[string]*character.Monster{"rat_b": ratB, "rat_a": ratA}},
			"cellar": {Monsters: map[string]*character.Monster{"wolf_1": wolf}},
		},
	}

	c, ok := gs.FindCombatant("felix")
	if !ok || c.Display != "Felix" || c.Stats != &gs.PC.Stats {
		t.Fatalf("PC = %q ok=%v stats=%p, want Felix live", c.Display, ok, c.Stats)
	}
	if _, ok := gs.FindCombatant("PC"); ok {
		t.Fatal("literal PC should not match a named PC")
	}

	c, ok = gs.FindCombatant("guard")
	if !ok || c.Display != "Guard Captain" || c.Stats != &gs.NPCs["guard"].Stats {
		t.Fatalf("NPC key = %q ok=%v", c.Display, ok)
	}
	c, ok = gs.FindCombatant("Guard Captain")
	if !ok || c.Stats != &gs.NPCs["guard"].Stats {
		t.Fatal("NPC name should resolve to the same live stats")
	}
	if _, ok := gs.FindCombatant("Pip Upton"); ok {
		t.Error("NPC in another room should not be found")
	}
	if _, ok := gs.FindCombatant("Bartender"); ok {
		t.Error("narrative NPC should not be an actor")
	}

	c, ok = gs.FindCombatant("rat_b")
	if !ok || c.Display != "Giant Rat" || c.Stats != &ratB.Stats {
		t.Fatalf("monster id = %q ok=%v hp=%d", c.Display, ok, hpOf(c.Stats))
	}
	c, ok = gs.FindCombatant("Giant Rat")
	if !ok || c.Stats != &ratA.Stats {
		t.Fatalf("duplicate name should take the lowest id, hp=%d", hpOf(c.Stats))
	}
	if _, ok := gs.FindCombatant("Wolf"); ok {
		t.Error("monster in another room should not be found")
	}

	gs.NPCs["guard"].IsDefeated = true
	if _, ok := gs.FindCombatant("Guard Captain"); ok {
		t.Fatal("defeated NPC should not be found")
	}
}

func TestFindCombatant_UnnamedPC(t *testing.T) {
	gs := &GameState{PC: &character.PC{HP: 8}}
	c, ok := gs.FindCombatant("PC")
	if !ok || c.Display != "PC" || c.Stats != &gs.PC.Stats {
		t.Fatalf("unnamed PC = %q ok=%v", c.Display, ok)
	}
	if _, ok := gs.FindCombatant(""); ok {
		t.Error("empty name should miss")
	}
	if _, ok := ((*GameState)(nil)).FindCombatant("PC"); ok {
		t.Error("nil gamestate should miss")
	}
}

func hpOf(s *character.Stats) int {
	if s == nil {
		return -1
	}
	return s.HP
}
