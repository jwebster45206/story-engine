package state

import (
	"log/slog"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestDeltaWorker_SyncFollowingNPCs_FollowPC(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "tavern",
		NPCs: map[string]actor.NPC{
			"gibbs": {
				Name:      "Gibbs",
				Location:  "ship",
				Following: "pc",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
			"ship":   {Name: "Ship"},
		},
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	worker.syncFollowingNPCs()

	// Gibbs should have moved to tavern (player's location)
	if gs.NPCs["gibbs"].Location != "tavern" {
		t.Errorf("Expected gibbs to be at tavern, got %s", gs.NPCs["gibbs"].Location)
	}
}

func TestDeltaWorker_SyncFollowingNPCs_FollowAnotherNPC(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "market",
		NPCs: map[string]actor.NPC{
			"captain": {
				Name:     "Captain",
				Location: "ship",
			},
			"guard": {
				Name:      "Guard",
				Location:  "market",
				Following: "captain",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"market": {Name: "Market"},
			"ship":   {Name: "Ship"},
		},
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	worker.syncFollowingNPCs()

	// Guard should have moved to ship (captain's location)
	if gs.NPCs["guard"].Location != "ship" {
		t.Errorf("Expected guard to be at ship, got %s", gs.NPCs["guard"].Location)
	}
}

func TestDeltaWorker_SyncFollowingNPCs_ChainedFollowing(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "tavern",
		NPCs: map[string]actor.NPC{
			"gibbs": {
				Name:      "Gibbs",
				Location:  "ship",
				Following: "pc",
			},
			"companion": {
				Name:      "Companion",
				Location:  "market",
				Following: "gibbs",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
			"ship":   {Name: "Ship"},
			"market": {Name: "Market"},
		},
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	worker.syncFollowingNPCs()

	// Gibbs should follow player to tavern
	if gs.NPCs["gibbs"].Location != "tavern" {
		t.Errorf("Expected gibbs to be at tavern, got %s", gs.NPCs["gibbs"].Location)
	}

	// Companion should follow Gibbs to tavern (synced in same call)
	if gs.NPCs["companion"].Location != "tavern" {
		t.Errorf("Expected companion to be at tavern, got %s", gs.NPCs["companion"].Location)
	}
}

func TestDeltaWorker_SyncFollowingNPCs_NoFollowing(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "tavern",
		NPCs: map[string]actor.NPC{
			"merchant": {
				Name:      "Merchant",
				Location:  "market",
				Following: "", // Not following anyone
			},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
			"market": {Name: "Market"},
		},
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	worker.syncFollowingNPCs()

	// Merchant should stay at market
	if gs.NPCs["merchant"].Location != "market" {
		t.Errorf("Expected merchant to stay at market, got %s", gs.NPCs["merchant"].Location)
	}
}

func TestDeltaWorker_SyncFollowingNPCs_TargetNotFound(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "tavern",
		NPCs: map[string]actor.NPC{
			"guard": {
				Name:      "Guard",
				Location:  "market",
				Following: "nonexistent_npc",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
			"market": {Name: "Market"},
		},
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	worker.syncFollowingNPCs()

	// Guard should stay at market (target not found)
	if gs.NPCs["guard"].Location != "market" {
		t.Errorf("Expected guard to stay at market, got %s", gs.NPCs["guard"].Location)
	}
}

func TestDeltaWorker_SyncFollowingNPCs_AlreadyAtTargetLocation(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "tavern",
		NPCs: map[string]actor.NPC{
			"gibbs": {
				Name:      "Gibbs",
				Location:  "tavern", // Already at player location
				Following: "pc",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
		},
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	worker.syncFollowingNPCs()

	// Gibbs should stay at tavern (no change needed)
	if gs.NPCs["gibbs"].Location != "tavern" {
		t.Errorf("Expected gibbs to stay at tavern, got %s", gs.NPCs["gibbs"].Location)
	}
}

func TestDeltaWorker_SyncFollowingNPCs_CaseInsensitiveMatch(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "tavern",
		NPCs: map[string]actor.NPC{
			"captain": {
				Name:     "Captain Morgan",
				Location: "ship",
			},
			"guard": {
				Name:      "Guard",
				Location:  "market",
				Following: "Captain Morgan", // Using display name instead of key
			},
		},
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
			"ship":   {Name: "Ship"},
			"market": {Name: "Market"},
		},
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	worker.syncFollowingNPCs()

	// Guard should have moved to ship (captain's location) via case-insensitive match
	if gs.NPCs["guard"].Location != "ship" {
		t.Errorf("Expected guard to be at ship, got %s", gs.NPCs["guard"].Location)
	}
}

func TestDeltaWorker_Apply_CallsSyncFollowingNPCs(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "market",
		NPCs: map[string]actor.NPC{
			"companion": {
				Name:      "Companion",
				Location:  "tavern",
				Following: "pc",
			},
		},
		WorldLocations: map[string]scenario.Location{
			"market": {Name: "Market"},
			"tavern": {Name: "Tavern"},
		},
	}
	delta := &conditionals.GameStateDelta{
		UserLocation: "market",
	}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	err := worker.Apply()
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Companion should have followed player to market
	if gs.NPCs["companion"].Location != "market" {
		t.Errorf("Expected companion to be at market after Apply(), got %s", gs.NPCs["companion"].Location)
	}
}
