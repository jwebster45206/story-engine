package state

import (
	"io"
	"log/slog"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestApplier_JustEntered(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	world := map[string]scenario.Location{
		"tavern": {Name: "Tavern"},
		"market": {Name: "Market"},
	}
	tests := []struct {
		name        string
		location    string
		justEntered bool
		delta       *conditionals.GameStateDelta
		wantLoc     string
		wantEntered bool
	}{
		{
			name:     "location change",
			location: "tavern",
			delta:    &conditionals.GameStateDelta{UserLocation: "market"},
			wantLoc:  "market", wantEntered: true,
		},
		{
			name:        "same location clears flag",
			location:    "tavern",
			justEntered: true,
			delta:       &conditionals.GameStateDelta{UserLocation: "tavern"},
			wantLoc:     "tavern",
		},
		{
			name:        "nil delta clears flag",
			location:    "tavern",
			justEntered: true,
			wantLoc:     "tavern",
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
				JustEntered:    tt.justEntered,
				WorldLocations: world,
			}
			if err := NewApplier(gs, tt.delta, &scenario.Scenario{}, logger).Apply(); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if gs.Location != tt.wantLoc {
				t.Errorf("Location = %q, want %q", gs.Location, tt.wantLoc)
			}
			if gs.JustEntered != tt.wantEntered {
				t.Errorf("JustEntered = %v, want %v", gs.JustEntered, tt.wantEntered)
			}
		})
	}
}
