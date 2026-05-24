package state

import (
	"log/slog"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestDeltaWorker_Apply_SetsJustEnteredOnLocationChange(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location: "tavern",
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
			"market": {Name: "Market"},
		},
	}
	delta := &conditionals.GameStateDelta{
		UserLocation: "market",
	}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	if err := worker.Apply(); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if gs.Location != "market" {
		t.Fatalf("expected Location to be 'market', got %q", gs.Location)
	}
	if !gs.JustEntered {
		t.Error("expected JustEntered=true after location change")
	}
}

func TestDeltaWorker_Apply_ClearsJustEnteredOnNoChange(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location:    "tavern",
		JustEntered: true, // carried over from a prior turn
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
		},
	}
	delta := &conditionals.GameStateDelta{
		// Same location as current - no movement this turn.
		UserLocation: "tavern",
	}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	if err := worker.Apply(); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if gs.JustEntered {
		t.Error("expected JustEntered=false when location did not change")
	}
}

func TestDeltaWorker_Apply_ClearsJustEnteredOnNilDelta(t *testing.T) {
	logger := slog.Default()
	gs := &GameState{
		Location:    "tavern",
		JustEntered: true,
	}

	worker := NewDeltaWorker(gs, nil, &scenario.Scenario{}, logger)
	if err := worker.Apply(); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if gs.JustEntered {
		t.Error("expected JustEntered=false after a nil-delta Apply")
	}
}

func TestDeltaWorker_Apply_JustEnteredFalseWhenUnresolvableLocation(t *testing.T) {
	// If the LLM returns a UserLocation that the engine can't resolve to a
	// known location key or name, the engine logs a warning and leaves
	// Location unchanged - that should NOT count as "just entered".
	logger := slog.Default()
	gs := &GameState{
		Location: "tavern",
		WorldLocations: map[string]scenario.Location{
			"tavern": {Name: "Tavern"},
		},
	}
	delta := &conditionals.GameStateDelta{
		UserLocation: "nonexistent_room",
	}
	s := &scenario.Scenario{}

	worker := NewDeltaWorker(gs, delta, s, logger)
	if err := worker.Apply(); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if gs.Location != "tavern" {
		t.Fatalf("expected Location to stay 'tavern', got %q", gs.Location)
	}
	if gs.JustEntered {
		t.Error("expected JustEntered=false when location resolution failed")
	}
}
