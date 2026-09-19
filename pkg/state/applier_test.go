package state

import (
	"io"
	"log/slog"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestApplier_LocationChange(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	world := map[string]scenario.Location{
		"tavern": {Name: "Tavern"},
		"market": {Name: "Market"},
	}
	tests := []struct {
		name     string
		location string
		delta    *conditionals.GameStateDelta
		wantLoc  string
	}{
		{
			name:     "location change",
			location: "tavern",
			delta:    &conditionals.GameStateDelta{UserLocation: "market"},
			wantLoc:  "market",
		},
		{
			name:     "same location is a no-op",
			location: "tavern",
			delta:    &conditionals.GameStateDelta{UserLocation: "tavern"},
			wantLoc:  "tavern",
		},
		{
			name:     "nil delta leaves location",
			location: "tavern",
			wantLoc:  "tavern",
		},
		{
			name:     "unresolvable location is not a change",
			location: "tavern",
			delta:    &conditionals.GameStateDelta{UserLocation: "nonexistent_room"},
			wantLoc:  "tavern",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := &GameState{
				Location:       tt.location,
				WorldLocations: world,
			}
			if err := NewApplier(gs, tt.delta, &scenario.Scenario{}, logger).Apply(); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if gs.Location != tt.wantLoc {
				t.Errorf("Location = %q, want %q", gs.Location, tt.wantLoc)
			}
		})
	}
}
