package worker

import (
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestApplyConditionalsCascade_NoConditionals(t *testing.T) {
	// Setup
	logger := slog.Default()
	gs := &state.GameState{
		ID:   uuid.New(),
		Vars: make(map[string]string),
	}
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{
		Scenes: map[string]scenario.Scene{
			"start": {
				Conditionals: map[string]scenario.Conditional{},
			},
		},
	}

	processor := &ChatProcessor{logger: logger}
	worker := state.NewDeltaWorker(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(worker, gs.ID)

	// No conditionals should trigger, function should return cleanly
	// (This is mainly testing that it doesn't panic or error)
}

func TestApplyConditionalsCascade_OneIteration(t *testing.T) {
	// Setup
	logger := slog.Default()
	gs := &state.GameState{
		ID:        uuid.New(),
		SceneName: "start",
		Vars: map[string]string{
			"player_score": "100",
		},
	}

	winFlag := true
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{
		Scenes: map[string]scenario.Scene{
			"start": {
				Conditionals: map[string]scenario.Conditional{
					"high_score_win": {
						When: conditionals.ConditionalWhen{
							Vars: map[string]string{
								"player_score": "100",
							},
						},
						Then: conditionals.GameStateDelta{
							GameEnded: &winFlag,
							SetVars: map[string]string{
								"victory": "true",
							},
						},
					},
				},
			},
		},
	}

	processor := &ChatProcessor{logger: logger}
	worker := state.NewDeltaWorker(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(worker, gs.ID)

	// Verify the conditional triggered and applied
	if gs.IsEnded != true {
		t.Errorf("Expected game to be ended, but IsEnded = %v", gs.IsEnded)
	}
	if victory := gs.Vars["victory"]; victory != "true" {
		t.Errorf("Expected victory var to be 'true', got %v", victory)
	}
}

func TestApplyConditionalsCascade_TwoIterations(t *testing.T) {
	// Setup
	logger := slog.Default()
	gs := &state.GameState{
		ID:        uuid.New(),
		SceneName: "start",
		Vars: map[string]string{
			"player_score": "100",
		},
	}

	endGame := true
	delta := &conditionals.GameStateDelta{}
	s := &scenario.Scenario{
		Scenes: map[string]scenario.Scene{
			"start": {
				Conditionals: map[string]scenario.Conditional{
					// First iteration: high score sets achievement
					"high_score_achievement": {
						When: conditionals.ConditionalWhen{
							Vars: map[string]string{
								"player_score": "100",
							},
						},
						Then: conditionals.GameStateDelta{
							SetVars: map[string]string{
								"achievement_unlocked": "true",
							},
						},
					},
					// Second iteration: achievement ends game
					"achievement_win": {
						When: conditionals.ConditionalWhen{
							Vars: map[string]string{
								"achievement_unlocked": "true",
							},
						},
						Then: conditionals.GameStateDelta{
							GameEnded: &endGame,
							SetVars: map[string]string{
								"victory": "true",
							},
						},
					},
				},
			},
		},
	}

	processor := &ChatProcessor{logger: logger}
	worker := state.NewDeltaWorker(gs, delta, s, logger)

	// Execute
	processor.applyConditionalsCascade(worker, gs.ID)

	// Verify both conditionals triggered in cascade
	if achievement := gs.Vars["achievement_unlocked"]; achievement != "true" {
		t.Errorf("Expected achievement_unlocked to be 'true', got %v", achievement)
	}
	if victory := gs.Vars["victory"]; victory != "true" {
		t.Errorf("Expected victory var to be 'true', got %v", victory)
	}
	if gs.IsEnded != true {
		t.Errorf("Expected game to be ended, but IsEnded = %v", gs.IsEnded)
	}
}
