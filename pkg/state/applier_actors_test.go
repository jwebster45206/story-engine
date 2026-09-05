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

func TestApplier_NPCEvents(t *testing.T) {
	tests := []struct {
		name      string
		playerLoc string
		npcs      map[string]actor.NPC
		events    []conditionals.NPCEvent
		want      map[string]actor.NPC
	}{
		{
			name: "set location",
			npcs: map[string]actor.NPC{"guard": {Name: "Guard", Location: "courtyard"}},
			events: []conditionals.NPCEvent{
				{NPCID: "guard", SetLocation: new("throne_room")},
			},
			want: map[string]actor.NPC{"guard": {Location: "throne_room"}},
		},
		{
			name:      "follow pc syncs location",
			playerLoc: "tavern",
			npcs:      map[string]actor.NPC{"companion": {Name: "Companion", Location: "market"}},
			events: []conditionals.NPCEvent{
				{NPCID: "companion", SetFollowing: new("pc")},
			},
			want: map[string]actor.NPC{"companion": {Location: "tavern", Following: "pc"}},
		},
		{
			name:      "clear following",
			playerLoc: "tavern",
			npcs:      map[string]actor.NPC{"companion": {Name: "Companion", Location: "tavern", Following: "pc"}},
			events: []conditionals.NPCEvent{
				{NPCID: "companion", SetFollowing: new("")},
			},
			want: map[string]actor.NPC{"companion": {Location: "tavern", Following: ""}},
		},
		{
			name:      "location and following on one event",
			playerLoc: "black_pearl",
			npcs:      map[string]actor.NPC{"gibbs": {Name: "Gibbs", Location: "tortuga"}},
			events: []conditionals.NPCEvent{
				{NPCID: "gibbs", SetLocation: new("black_pearl"), SetFollowing: new("pc")},
			},
			want: map[string]actor.NPC{"gibbs": {Location: "black_pearl", Following: "pc"}},
		},
		{
			name: "unknown npc is a no-op",
			npcs: map[string]actor.NPC{"guard": {Name: "Guard", Location: "courtyard"}},
			events: []conditionals.NPCEvent{
				{NPCID: "nonexistent", SetLocation: new("courtyard")},
			},
			want: map[string]actor.NPC{"guard": {Location: "courtyard"}},
		},
		{
			name: "unknown location is a no-op",
			npcs: map[string]actor.NPC{"guard": {Name: "Guard", Location: "courtyard"}},
			events: []conditionals.NPCEvent{
				{NPCID: "guard", SetLocation: new("nonexistent_location")},
			},
			want: map[string]actor.NPC{"guard": {Location: "courtyard"}},
		},
		{
			name: "unknown following target is still recorded",
			npcs: map[string]actor.NPC{"companion": {Name: "Companion", Location: "market"}},
			events: []conditionals.NPCEvent{
				{NPCID: "companion", SetFollowing: new("nonexistent_npc")},
			},
			want: map[string]actor.NPC{"companion": {Location: "market", Following: "nonexistent_npc"}},
		},
		{
			name: "match npc and location by display name",
			npcs: map[string]actor.NPC{"guard": {Name: "Royal Guard", Location: "courtyard"}},
			events: []conditionals.NPCEvent{
				{NPCID: "Royal Guard", SetLocation: new("Throne Room")},
			},
			want: map[string]actor.NPC{"guard": {Location: "throne_room"}},
		},
	}

	world := map[string]scenario.Location{
		"courtyard":   {Name: "Courtyard"},
		"throne_room": {Name: "Throne Room"},
		"market":      {Name: "Market"},
		"tavern":      {Name: "Tavern"},
		"tortuga":     {Name: "Tortuga"},
		"black_pearl": {Name: "Black Pearl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := &GameState{
				Location:       tt.playerLoc,
				NPCs:           tt.npcs,
				WorldLocations: world,
			}
			delta := &conditionals.GameStateDelta{NPCEvents: tt.events}
			if err := NewApplier(gs, delta, nil, nil).Apply(); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			for id, want := range tt.want {
				got := gs.NPCs[id]
				if got.Location != want.Location {
					t.Errorf("%s.Location = %q, want %q", id, got.Location, want.Location)
				}
				if got.Following != want.Following {
					t.Errorf("%s.Following = %q, want %q", id, got.Following, want.Following)
				}
			}
		})
	}
}

func TestApplier_SyncFollowing(t *testing.T) {
	tests := []struct {
		name      string
		playerLoc string
		npcs      map[string]actor.NPC
		wantLoc   map[string]string
	}{
		{
			name:      "follow pc",
			playerLoc: "tavern",
			npcs: map[string]actor.NPC{
				"gibbs": {Name: "Gibbs", Location: "ship", Following: "pc"},
			},
			wantLoc: map[string]string{"gibbs": "tavern"},
		},
		{
			name:      "follow another npc",
			playerLoc: "market",
			npcs: map[string]actor.NPC{
				"captain": {Name: "Captain", Location: "ship"},
				"guard":   {Name: "Guard", Location: "market", Following: "captain"},
			},
			wantLoc: map[string]string{"guard": "ship"},
		},
		{
			name:      "chained following converges",
			playerLoc: "tavern",
			npcs: map[string]actor.NPC{
				"gibbs":     {Name: "Gibbs", Location: "ship", Following: "pc"},
				"companion": {Name: "Companion", Location: "market", Following: "gibbs"},
			},
			wantLoc: map[string]string{"gibbs": "tavern", "companion": "tavern"},
		},
		{
			name:      "missing target stays put",
			playerLoc: "tavern",
			npcs: map[string]actor.NPC{
				"guard": {Name: "Guard", Location: "market", Following: "nonexistent_npc"},
			},
			wantLoc: map[string]string{"guard": "market"},
		},
		{
			name:      "follow by display name",
			playerLoc: "tavern",
			npcs: map[string]actor.NPC{
				"captain": {Name: "Captain Morgan", Location: "ship"},
				"guard":   {Name: "Guard", Location: "market", Following: "Captain Morgan"},
			},
			wantLoc: map[string]string{"guard": "ship"},
		},
	}

	world := map[string]scenario.Location{
		"tavern": {Name: "Tavern"},
		"ship":   {Name: "Ship"},
		"market": {Name: "Market"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := &GameState{
				Location:       tt.playerLoc,
				NPCs:           tt.npcs,
				WorldLocations: world,
			}
			NewApplier(gs, &conditionals.GameStateDelta{}, &scenario.Scenario{}, nil).syncFollowingNPCs()
			for id, want := range tt.wantLoc {
				if got := gs.NPCs[id].Location; got != want {
					t.Errorf("%s.Location = %q, want %q", id, got, want)
				}
			}
		})
	}
}

func TestApplier_ApplySyncsFollowers(t *testing.T) {
	gs := &GameState{
		Location: "market",
		NPCs: map[string]actor.NPC{
			"companion": {Name: "Companion", Location: "tavern", Following: "pc"},
		},
		WorldLocations: map[string]scenario.Location{
			"market": {Name: "Market"},
			"tavern": {Name: "Tavern"},
		},
	}
	if err := NewApplier(gs, &conditionals.GameStateDelta{UserLocation: "market"}, &scenario.Scenario{}, nil).Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := gs.NPCs["companion"].Location; got != "market" {
		t.Errorf("companion.Location = %q, want market", got)
	}
}

type mockMonsterStorage struct {
	monsters map[string]*actor.Monster
}

func (m *mockMonsterStorage) GetMonster(_ context.Context, templateID string) (*actor.Monster, error) {
	return m.monsters[templateID], nil
}

func TestApplier_MonsterSpawn(t *testing.T) {
	gs := &GameState{
		WorldLocations: map[string]scenario.Location{
			"cellar": {Name: "Dark Cellar", Monsters: map[string]*actor.Monster{}},
		},
	}
	storage := &mockMonsterStorage{
		monsters: map[string]*actor.Monster{
			"giant_rat": {ID: "giant_rat", Name: "Giant Rat", AC: 8, HP: 4, MaxHP: 4},
		},
	}
	delta := &conditionals.GameStateDelta{
		MonsterEvents: []conditionals.MonsterEvent{
			{Action: "spawn", InstanceID: "rat_1", Template: "giant_rat", Location: "cellar"},
		},
	}
	if err := NewApplier(gs, delta, nil, nil).WithStorage(storage).WithContext(context.Background()).Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	monster, ok := gs.WorldLocations["cellar"].Monsters["rat_1"]
	if !ok {
		t.Fatal("monster was not spawned")
	}
	if monster.Name != "Giant Rat" {
		t.Errorf("Name = %q, want Giant Rat", monster.Name)
	}
}

func TestApplier_MonsterDespawn(t *testing.T) {
	gs := &GameState{
		WorldLocations: map[string]scenario.Location{
			"cellar": {
				Name: "Dark Cellar",
				Monsters: map[string]*actor.Monster{
					"rat_1": {
						ID: "rat_1", Name: "Giant Rat", Location: "cellar",
						Items: []string{"rat_pelt"}, DropItemsOnDefeat: true,
					},
				},
			},
		},
	}
	delta := &conditionals.GameStateDelta{
		MonsterEvents: []conditionals.MonsterEvent{
			{Action: "despawn", InstanceID: "rat_1"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := NewApplier(gs, delta, nil, logger).Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, exists := gs.WorldLocations["cellar"].Monsters["rat_1"]; exists {
		t.Error("monster was not despawned")
	}
	if items := gs.WorldLocations["cellar"].Items; len(items) != 1 || items[0] != "rat_pelt" {
		t.Errorf("Items = %v, want [rat_pelt]", items)
	}
}
