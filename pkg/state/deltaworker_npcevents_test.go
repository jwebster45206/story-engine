package state

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestDeltaWorker_HandleNPCEvent_SetLocation(t *testing.T) {
	gs := &GameState{
		NPCs: map[string]actor.NPC{
			"guard": {
				Name:     "Guard",
				Location: "courtyard",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"courtyard":   {Name: "Courtyard"},
			"throne_room": {Name: "Throne Room"},
		},
	}

	delta := &conditionals.GameStateDelta{
		NPCEvents: []conditionals.NPCEvent{
			{
				NPCID:       "guard",
				SetLocation: stringPtr("throne_room"),
			},
		},
	}

	dw := NewDeltaWorker(gs, delta, nil, nil)
	err := dw.Apply()
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if gs.NPCs["guard"].Location != "throne_room" {
		t.Errorf("Expected guard location to be throne_room, got %s", gs.NPCs["guard"].Location)
	}
}

func TestDeltaWorker_HandleNPCEvent_SetFollowing(t *testing.T) {
	gs := &GameState{
		Location: "tavern",
		NPCs: map[string]actor.NPC{
			"companion": {
				Name:      "Loyal Companion",
				Location:  "market",
				Following: "",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"market": {Name: "Market"},
			"tavern": {Name: "Tavern"},
		},
	}

	delta := &conditionals.GameStateDelta{
		NPCEvents: []conditionals.NPCEvent{
			{
				NPCID:        "companion",
				SetFollowing: stringPtr("pc"),
			},
		},
	}

	dw := NewDeltaWorker(gs, delta, nil, nil)
	err := dw.Apply()
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if gs.NPCs["companion"].Following != "pc" {
		t.Errorf("Expected companion following to be pc, got %s", gs.NPCs["companion"].Following)
	}

	// Following sync should have moved the companion to player location
	if gs.NPCs["companion"].Location != "tavern" {
		t.Errorf("Expected companion location to be tavern (synced), got %s", gs.NPCs["companion"].Location)
	}
}

func TestDeltaWorker_HandleNPCEvent_ClearFollowing(t *testing.T) {
	gs := &GameState{
		Location: "tavern",
		NPCs: map[string]actor.NPC{
			"companion": {
				Name:      "Loyal Companion",
				Location:  "tavern",
				Following: "pc",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
		},
	}

	delta := &conditionals.GameStateDelta{
		NPCEvents: []conditionals.NPCEvent{
			{
				NPCID:        "companion",
				SetFollowing: stringPtr(""),
			},
		},
	}

	dw := NewDeltaWorker(gs, delta, nil, nil)
	err := dw.Apply()
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if gs.NPCs["companion"].Following != "" {
		t.Errorf("Expected companion following to be empty, got %s", gs.NPCs["companion"].Following)
	}
}

func TestDeltaWorker_HandleNPCEvent_CombinedLocationAndFollowing(t *testing.T) {
	gs := &GameState{
		Location: "black_pearl",
		NPCs: map[string]actor.NPC{
			"gibbs": {
				Name:      "Gibbs",
				Location:  "tortuga",
				Following: "",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"tortuga":     {Name: "Tortuga"},
			"black_pearl": {Name: "Black Pearl"},
		},
	}

	delta := &conditionals.GameStateDelta{
		NPCEvents: []conditionals.NPCEvent{
			{
				NPCID:        "gibbs",
				SetLocation:  stringPtr("black_pearl"),
				SetFollowing: stringPtr("pc"),
			},
		},
	}

	dw := NewDeltaWorker(gs, delta, nil, nil)
	err := dw.Apply()
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if gs.NPCs["gibbs"].Location != "black_pearl" {
		t.Errorf("Expected gibbs location to be black_pearl, got %s", gs.NPCs["gibbs"].Location)
	}

	if gs.NPCs["gibbs"].Following != "pc" {
		t.Errorf("Expected gibbs following to be pc, got %s", gs.NPCs["gibbs"].Following)
	}
}

func TestDeltaWorker_HandleNPCEvent_InvalidNPC(t *testing.T) {
	gs := &GameState{
		NPCs: map[string]actor.NPC{
			"guard": {
				Name:     "Guard",
				Location: "courtyard",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"courtyard": {Name: "Courtyard"},
		},
	}

	delta := &conditionals.GameStateDelta{
		NPCEvents: []conditionals.NPCEvent{
			{
				NPCID:       "nonexistent",
				SetLocation: stringPtr("courtyard"),
			},
		},
	}

	dw := NewDeltaWorker(gs, delta, nil, nil)
	err := dw.Apply()
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Should not crash, just log warning
	if gs.NPCs["guard"].Location != "courtyard" {
		t.Errorf("Guard location should remain unchanged")
	}
}

func TestDeltaWorker_HandleNPCEvent_InvalidLocation(t *testing.T) {
	gs := &GameState{
		NPCs: map[string]actor.NPC{
			"guard": {
				Name:     "Guard",
				Location: "courtyard",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"courtyard": {Name: "Courtyard"},
		},
	}

	delta := &conditionals.GameStateDelta{
		NPCEvents: []conditionals.NPCEvent{
			{
				NPCID:       "guard",
				SetLocation: stringPtr("nonexistent_location"),
			},
		},
	}

	dw := NewDeltaWorker(gs, delta, nil, nil)
	err := dw.Apply()
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Location should remain unchanged due to invalid target
	if gs.NPCs["guard"].Location != "courtyard" {
		t.Errorf("Expected guard location to remain courtyard, got %s", gs.NPCs["guard"].Location)
	}
}

func TestDeltaWorker_HandleNPCEvent_InvalidFollowingTarget(t *testing.T) {
	gs := &GameState{
		NPCs: map[string]actor.NPC{
			"companion": {
				Name:      "Companion",
				Location:  "market",
				Following: "",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"market": {Name: "Market"},
		},
	}

	delta := &conditionals.GameStateDelta{
		NPCEvents: []conditionals.NPCEvent{
			{
				NPCID:        "companion",
				SetFollowing: stringPtr("nonexistent_npc"),
			},
		},
	}

	dw := NewDeltaWorker(gs, delta, nil, nil)
	err := dw.Apply()
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Following should still be set (warning logged but not blocked)
	if gs.NPCs["companion"].Following != "nonexistent_npc" {
		t.Errorf("Expected following to be set to nonexistent_npc (with warning), got %s", gs.NPCs["companion"].Following)
	}
}

func TestDeltaWorker_HandleNPCEvent_CaseInsensitiveMatching(t *testing.T) {
	gs := &GameState{
		NPCs: map[string]actor.NPC{
			"guard": {
				Name:     "Royal Guard",
				Location: "courtyard",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"courtyard":   {Name: "Courtyard"},
			"throne_room": {Name: "Throne Room"},
		},
	}

	delta := &conditionals.GameStateDelta{
		NPCEvents: []conditionals.NPCEvent{
			{
				NPCID:       "Royal Guard",            // Match by name
				SetLocation: stringPtr("Throne Room"), // Match by location name
			},
		},
	}

	dw := NewDeltaWorker(gs, delta, nil, nil)
	err := dw.Apply()
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if gs.NPCs["guard"].Location != "throne_room" {
		t.Errorf("Expected guard location to be throne_room (case-insensitive match), got %s", gs.NPCs["guard"].Location)
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
