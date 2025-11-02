package state

import (
	"testing"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestSpawnMonster(t *testing.T) {
	gs := &GameState{
		WorldLocations: map[string]scenario.Location{
			"cellar": {Name: "Cellar"},
		},
	}
	template := &actor.Monster{Name: "Rat", AC: 12, MaxHP: 9}
	m := gs.SpawnMonster("rat_1", template, "cellar")
	if m == nil {
		t.Fatal("SpawnMonster returned nil")
	}
	loc := gs.WorldLocations["cellar"]
	if loc.Monsters["rat_1"] == nil {
		t.Error("monster not added to location")
	}
}

func TestDespawnMonster(t *testing.T) {
	rat := &actor.Monster{ID: "rat_1", Location: "cellar"}
	gs := &GameState{
		WorldLocations: map[string]scenario.Location{
			"cellar": {Name: "Cellar", Monsters: map[string]*actor.Monster{"rat_1": rat}},
		},
	}
	gs.DespawnMonster("rat_1")
	loc := gs.WorldLocations["cellar"]
	if _, exists := loc.Monsters["rat_1"]; exists {
		t.Error("monster not removed")
	}
}

func TestEvaluateDefeats(t *testing.T) {
	rat1 := &actor.Monster{ID: "rat_1", HP: 0, MaxHP: 9}
	rat2 := &actor.Monster{ID: "rat_2", HP: 5, MaxHP: 9}
	gs := &GameState{
		WorldLocations: map[string]scenario.Location{
			"cellar": {Name: "Cellar", Monsters: map[string]*actor.Monster{"rat_1": rat1, "rat_2": rat2}},
		},
	}
	gs.EvaluateDefeats()
	loc := gs.WorldLocations["cellar"]
	if _, exists := loc.Monsters["rat_1"]; exists {
		t.Error("defeated monster not removed")
	}
	if _, exists := loc.Monsters["rat_2"]; !exists {
		t.Error("alive monster incorrectly removed")
	}
}

