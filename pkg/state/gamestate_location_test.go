package state

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestGameState_GetContingencyPrompts_WithLocations(t *testing.T) {
	tests := []struct {
		name              string
		gameState         *GameState
		scenario          *scenario.Scenario
		expectedPrompts   []string
		unexpectedPrompts []string
	}{
		{
			name: "Location at current location shows prompts",
			gameState: &GameState{
				Location: "tavern",
				WorldLocations: map[string]scenario.Location{
					"tavern": {
						Name: "Tavern",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The tavern has a secret escape hatch"},
							{Prompt: "The atmosphere is warm and welcoming"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			expectedPrompts: []string{
				"The tavern has a secret escape hatch",
				"The atmosphere is warm and welcoming",
			},
		},
		{
			name: "Location at different location does not show prompts",
			gameState: &GameState{
				Location: "market",
				WorldLocations: map[string]scenario.Location{
					"tavern": {
						Name: "Tavern",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The tavern has a secret escape hatch"},
						},
					},
					"market": {
						Name: "Market",
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			unexpectedPrompts: []string{
				"The tavern has a secret escape hatch",
			},
		},
		{
			name: "Location conditional prompts based on vars",
			gameState: &GameState{
				Location: "library",
				Vars: map[string]string{
					"discovered_secret": "true",
					"librarian_trust":   "high",
				},
				WorldLocations: map[string]scenario.Location{
					"library": {
						Name: "Library",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The library is vast and quiet"},
							{
								Prompt: "You notice a hidden door behind the bookshelf",
								When: &conditionals.ConditionalWhen{
									Vars: map[string]string{"discovered_secret": "true"},
								},
							},
							{
								Prompt: "The librarian has marked certain books for you",
								When: &conditionals.ConditionalWhen{
									Vars: map[string]string{
										"discovered_secret": "true",
										"librarian_trust":   "high",
									},
								},
							},
							{
								Prompt: "The restricted section is locked",
								When: &conditionals.ConditionalWhen{
									Vars: map[string]string{"discovered_secret": "false"},
								},
							},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			expectedPrompts: []string{
				"The library is vast and quiet",
				"You notice a hidden door behind the bookshelf",
				"The librarian has marked certain books for you",
			},
			unexpectedPrompts: []string{
				"The restricted section is locked",
			},
		},
		{
			name: "Multiple locations with prompts",
			gameState: &GameState{
				Location: "plaza",
				WorldLocations: map[string]scenario.Location{
					"plaza": {
						Name: "Plaza",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The plaza fountain shows mystical symbols"},
						},
					},
					"temple": {
						Name: "Temple",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The temple holds ancient relics"},
						},
					},
					"market": {
						Name: "Market",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "Merchants sell exotic wares"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			expectedPrompts: []string{
				"The plaza fountain shows mystical symbols",
			},
			unexpectedPrompts: []string{
				"The temple holds ancient relics",
				"Merchants sell exotic wares",
			},
		},
		{
			name: "Location prompts combined with scenario, scene, PC, and NPC prompts",
			gameState: &GameState{
				Location:  "inn",
				SceneName: "arrival",
				WorldLocations: map[string]scenario.Location{
					"inn": {
						Name: "Inn",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "Location prompt: The inn has a warm fireplace"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
				ContingencyPrompts: []conditionals.ContingencyPrompt{
					{Prompt: "Scenario-level prompt"},
				},
				Scenes: map[string]scenario.Scene{
					"arrival": {
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "Scene-level prompt"},
						},
					},
				},
			},
			expectedPrompts: []string{
				"Scenario-level prompt",
				"Scene-level prompt",
				"Location prompt: The inn has a warm fireplace",
			},
		},
		{
			name: "Location prompts with turn counter conditions",
			gameState: &GameState{
				Location:         "cave",
				TurnCounter:      10,
				SceneTurnCounter: 3,
				WorldLocations: map[string]scenario.Location{
					"cave": {
						Name: "Cave",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The cave is dark and damp"},
							{
								Prompt: "Your eyes have adjusted to the darkness",
								When: &conditionals.ConditionalWhen{
									MinTurns: func() *int { i := 5; return &i }(),
								},
							},
							{
								Prompt: "You've been in this cave section long enough to notice details",
								When: &conditionals.ConditionalWhen{
									MinSceneTurns: func() *int { i := 2; return &i }(),
								},
							},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			expectedPrompts: []string{
				"The cave is dark and damp",
				"Your eyes have adjusted to the darkness",
				"You've been in this cave section long enough to notice details",
			},
		},
		{
			name: "No prompts when location not in WorldLocations",
			gameState: &GameState{
				Location:       "nowhere",
				WorldLocations: map[string]scenario.Location{},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			expectedPrompts: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompts := tt.gameState.GetContingencyPrompts(tt.scenario)

			// Check expected prompts are present
			for _, expected := range tt.expectedPrompts {
				found := false
				for _, prompt := range prompts {
					if prompt == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected prompt not found: %q", expected)
				}
			}

			// Check unexpected prompts are absent
			for _, unexpected := range tt.unexpectedPrompts {
				for _, prompt := range prompts {
					if prompt == unexpected {
						t.Errorf("Unexpected prompt found: %q", unexpected)
					}
				}
			}
		})
	}
}
