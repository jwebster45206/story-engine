package state

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// mockMonsterStorage is a mock implementation of MonsterStorage for testing
type mockMonsterStorage struct {
	monsters map[string]*actor.Monster
}

var noopLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

func (m *mockMonsterStorage) GetMonster(ctx context.Context, templateID string) (*actor.Monster, error) {
	if monster, ok := m.monsters[templateID]; ok {
		return monster, nil
	}
	return nil, nil
}

func TestDeltaWorker_MonsterSpawn(t *testing.T) {
	// Setup game state with a location
	gs := &GameState{
		WorldLocations: map[string]scenario.Location{
			"cellar": {
				Name:        "Dark Cellar",
				Description: "A dank underground room",
				Monsters:    map[string]*actor.Monster{},
			},
		},
	}

	// Create mock storage with a monster template
	storage := &mockMonsterStorage{
		monsters: map[string]*actor.Monster{
			"giant_rat": {
				ID:          "giant_rat",
				Name:        "Giant Rat",
				Description: "A filthy rodent",
				AC:          8,
				HP:          4,
				MaxHP:       4,
			},
		},
	}

	// Create delta with monster spawn
	delta := &conditionals.GameStateDelta{
		MonsterEvents: []conditionals.MonsterEvent{
			{
				Action:     "spawn",
				InstanceID: "rat_1",
				Template:   "giant_rat",
				Location:   "cellar",
			},
		},
	}

	// Apply the delta
	dw := NewDeltaWorker(gs, delta, nil, nil).
		WithStorage(storage).
		WithContext(context.Background())

	if err := dw.Apply(); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify monster was spawned
	loc := gs.WorldLocations["cellar"]
	monster, exists := loc.Monsters["rat_1"]
	if !exists {
		t.Fatal("Monster was not spawned")
	}

	if monster.Name != "Giant Rat" {
		t.Errorf("Expected monster name 'Giant Rat', got '%s'", monster.Name)
	}

	if monster.Location != "cellar" {
		t.Errorf("Expected monster location 'cellar', got '%s'", monster.Location)
	}
}

func TestDeltaWorker_MonsterDespawn(t *testing.T) {
	// Setup game state with a monster already spawned
	gs := &GameState{
		WorldLocations: map[string]scenario.Location{
			"cellar": {
				Name:        "Dark Cellar",
				Description: "A dank underground room",
				Monsters: map[string]*actor.Monster{
					"rat_1": {
						ID:                "rat_1",
						Name:              "Giant Rat",
						Location:          "cellar",
						AC:                8,
						HP:                4,
						MaxHP:             4,
						Items:             []string{"rat_pelt"},
						DropItemsOnDefeat: true,
					},
				},
			},
		},
	}

	// Create delta with monster despawn
	delta := &conditionals.GameStateDelta{
		MonsterEvents: []conditionals.MonsterEvent{
			{
				Action:     "despawn",
				InstanceID: "rat_1",
			},
		},
	}
	dw := NewDeltaWorker(gs, delta, nil, noopLogger)
	if err := dw.Apply(); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify monster was despawned
	loc := gs.WorldLocations["cellar"]
	if _, exists := loc.Monsters["rat_1"]; exists {
		t.Error("Monster was not despawned")
	}

	// Verify items were dropped
	if len(loc.Items) != 1 || loc.Items[0] != "rat_pelt" {
		t.Errorf("Expected items ['rat_pelt'], got %v", loc.Items)
	}
}
