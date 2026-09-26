package state

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestFindActor(t *testing.T) {
	ratA := &character.Monster{ID: "rat_a", Name: "Giant Rat", Stats: character.Stats{HP: 4, AC: 8}}
	ratB := &character.Monster{ID: "rat_b", Name: "Giant Rat", Stats: character.Stats{HP: 9, AC: 12}}
	wolf := &character.Monster{ID: "wolf_1", Name: "Wolf", Stats: character.Stats{HP: 11}}
	gs := &GameState{
		Location: "tavern",
		PC:       &character.PC{Name: "Felix", Stats: character.Stats{HP: 12, AC: 14}},
		NPCs: map[string]*character.NPC{
			"guard":     {Name: "Guard Captain", Location: "tavern", Stats: character.Stats{HP: 10, AC: 16}},
			"pip":       {Name: "Pip Upton", Location: "cellar", Stats: character.Stats{HP: 5}},
			"bartender": {Name: "Bartender", Location: "tavern"},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Monsters: map[string]*character.Monster{"rat_b": ratB, "rat_a": ratA}},
			"cellar": {Monsters: map[string]*character.Monster{"wolf_1": wolf}},
		},
	}

	stats, display, ok := gs.FindActor("felix")
	if !ok || display != "Felix" || stats != &gs.PC.Stats {
		t.Fatalf("PC = %q ok=%v stats=%p, want Felix live", display, ok, stats)
	}
	if _, _, ok := gs.FindActor("PC"); ok {
		t.Fatal("literal PC should not match a named PC")
	}

	stats, display, ok = gs.FindActor("guard")
	if !ok || display != "Guard Captain" || stats != &gs.NPCs["guard"].Stats {
		t.Fatalf("NPC key = %q ok=%v", display, ok)
	}
	stats, _, ok = gs.FindActor("Guard Captain")
	if !ok || stats != &gs.NPCs["guard"].Stats {
		t.Fatal("NPC name should resolve to the same live stats")
	}
	if _, _, ok := gs.FindActor("Pip Upton"); ok {
		t.Error("NPC in another room should not be found")
	}
	if _, _, ok := gs.FindActor("Bartender"); ok {
		t.Error("narrative NPC should not be an actor")
	}

	stats, display, ok = gs.FindActor("rat_b")
	if !ok || display != "Giant Rat" || stats != &ratB.Stats {
		t.Fatalf("monster id = %q ok=%v hp=%d", display, ok, hpOf(stats))
	}
	stats, _, ok = gs.FindActor("Giant Rat")
	if !ok || stats != &ratA.Stats {
		t.Fatalf("duplicate name should take the lowest id, hp=%d", hpOf(stats))
	}
	if _, _, ok := gs.FindActor("Wolf"); ok {
		t.Error("monster in another room should not be found")
	}
}

func TestFindActor_UnnamedPC(t *testing.T) {
	gs := &GameState{PC: &character.PC{Stats: character.Stats{HP: 8}}}
	stats, display, ok := gs.FindActor("PC")
	if !ok || display != "PC" || stats != &gs.PC.Stats {
		t.Fatalf("unnamed PC = %q ok=%v", display, ok)
	}
	if _, _, ok := gs.FindActor(""); ok {
		t.Error("empty name should miss")
	}
	if _, _, ok := ((*GameState)(nil)).FindActor("PC"); ok {
		t.Error("nil gamestate should miss")
	}
}

func hpOf(s *character.Stats) int {
	if s == nil {
		return -1
	}
	return s.HP
}
