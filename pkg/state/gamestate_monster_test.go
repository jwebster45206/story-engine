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
	monsterDef := &actor.Monster{ID: "rat_1", TemplateID: "giant_rat", Location: "cellar"}
	m := gs.SpawnMonster(template, monsterDef)
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
