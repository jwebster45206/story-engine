package scenario

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// mockGameStateView implements conditionals.GameStateView for testing
type mockGameStateView struct {
	sceneName        string
	vars             map[string]string
	sceneTurnCounter int
	turnCounter      int
	userLocation     string
}

func (m *mockGameStateView) GetSceneName() string       { return m.sceneName }
func (m *mockGameStateView) GetVars() map[string]string { return m.vars }
func (m *mockGameStateView) GetSceneTurnCounter() int   { return m.sceneTurnCounter }
func (m *mockGameStateView) GetTurnCounter() int        { return m.turnCounter }
func (m *mockGameStateView) GetUserLocation() string    { return m.userLocation }

func TestFilterContingencyPrompts(t *testing.T) {
	tests := []struct {
		name     string
		prompts  []conditionals.ContingencyPrompt
		gsView   conditionals.GameStateView
		expected []string
	}{
		{
			name:     "no prompts",
			prompts:  []conditionals.ContingencyPrompt{},
			gsView:   &mockGameStateView{},
			expected: []string{},
		},
		{
			name: "prompt without condition",
			prompts: []conditionals.ContingencyPrompt{
				{Prompt: "Always show this"},
			},
			gsView:   &mockGameStateView{},
			expected: []string{"Always show this"},
		},
		{
			name: "multiple prompts without conditions",
			prompts: []conditionals.ContingencyPrompt{
				{Prompt: "Prompt 1"},
				{Prompt: "Prompt 2"},
				{Prompt: "Prompt 3"},
			},
			gsView:   &mockGameStateView{},
			expected: []string{"Prompt 1", "Prompt 2", "Prompt 3"},
		},
		{
			name: "prompt with satisfied var condition",
			prompts: []conditionals.ContingencyPrompt{
				{
					Prompt: "Show when has_key is true",
					When: &conditionals.ConditionalWhen{
						Vars: map[string]string{"has_key": "true"},
					},
				},
			},
			gsView: &mockGameStateView{
				vars: map[string]string{"has_key": "true"},
			},
			expected: []string{"Show when has_key is true"},
		},
		{
			name: "prompt with unsatisfied var condition",
			prompts: []conditionals.ContingencyPrompt{
				{
					Prompt: "Show when has_key is true",
					When: &conditionals.ConditionalWhen{
						Vars: map[string]string{"has_key": "true"},
					},
				},
			},
			gsView: &mockGameStateView{
				vars: map[string]string{"has_key": "false"},
			},
			expected: []string{},
		},
		{
			name: "mixed conditional and unconditional prompts",
			prompts: []conditionals.ContingencyPrompt{
				{Prompt: "Always show"},
				{
					Prompt: "Show when has_item is true",
					When: &conditionals.ConditionalWhen{
						Vars: map[string]string{"has_item": "true"},
					},
				},
				{Prompt: "Also always show"},
			},
			gsView: &mockGameStateView{
				vars: map[string]string{"has_item": "false"},
			},
			expected: []string{"Always show", "Also always show"},
		},
		{
			name: "turn counter condition satisfied",
			prompts: []conditionals.ContingencyPrompt{
				{
					Prompt: "Show on turn 5",
					When: &conditionals.ConditionalWhen{
						TurnCounter: intPtr(5),
					},
				},
			},
			gsView: &mockGameStateView{
				turnCounter: 5,
			},
			expected: []string{"Show on turn 5"},
		},
		{
			name: "turn counter condition not satisfied",
			prompts: []conditionals.ContingencyPrompt{
				{
					Prompt: "Show on turn 5",
					When: &conditionals.ConditionalWhen{
						TurnCounter: intPtr(5),
					},
				},
			},
			gsView: &mockGameStateView{
				turnCounter: 3,
			},
			expected: []string{},
		},
		{
			name: "scene turn counter with min threshold satisfied",
			prompts: []conditionals.ContingencyPrompt{
				{
					Prompt: "Show after 3 scene turns",
					When: &conditionals.ConditionalWhen{
						MinSceneTurns: intPtr(3),
					},
				},
			},
			gsView: &mockGameStateView{
				sceneTurnCounter: 5,
			},
			expected: []string{"Show after 3 scene turns"},
		},
		{
			name: "scene turn counter with min threshold not satisfied",
			prompts: []conditionals.ContingencyPrompt{
				{
					Prompt: "Show after 3 scene turns",
					When: &conditionals.ConditionalWhen{
						MinSceneTurns: intPtr(3),
					},
				},
			},
			gsView: &mockGameStateView{
				sceneTurnCounter: 2,
			},
			expected: []string{},
		},
		{
			name: "multiple prompts with different conditions",
			prompts: []conditionals.ContingencyPrompt{
				{
					Prompt: "Show when has_sword",
					When: &conditionals.ConditionalWhen{
						Vars: map[string]string{"has_sword": "true"},
					},
				},
				{
					Prompt: "Show when has_shield",
					When: &conditionals.ConditionalWhen{
						Vars: map[string]string{"has_shield": "true"},
					},
				},
				{Prompt: "Always show"},
			},
			gsView: &mockGameStateView{
				vars: map[string]string{
					"has_sword":  "true",
					"has_shield": "false",
				},
			},
			expected: []string{"Show when has_sword", "Always show"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterContingencyPrompts(tt.prompts, tt.gsView)

			if len(result) != len(tt.expected) {
				t.Errorf("FilterContingencyPrompts() returned %d prompts, expected %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Expected: %v", tt.expected)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("FilterContingencyPrompts()[%d] = %q, expected %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}

func TestScenario_EvaluateStoryEvents(t *testing.T) {
	tests := []struct {
		name     string
		scenario *Scenario
		gsView   conditionals.GameStateView
		expected []string // Expected event keys
	}{
		{
			name: "no scene",
			scenario: &Scenario{
				Scenes: map[string]Scene{},
			},
			gsView:   &mockGameStateView{sceneName: ""},
			expected: nil,
		},
		{
			name: "scene with no story events",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"test_scene": {
						StoryEvents: map[string]StoryEvent{},
					},
				},
			},
			gsView:   &mockGameStateView{sceneName: "test_scene"},
			expected: nil,
		},
		{
			name: "story event triggered by variable",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						StoryEvents: map[string]StoryEvent{
							"dracula_appears": {
								When: conditionals.ConditionalWhen{
									Vars: map[string]string{"opened_grimoire": "true"},
								},
								Prompt: "Count Dracula materializes from the shadows.",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName: "castle",
				vars:      map[string]string{"opened_grimoire": "true"},
			},
			expected: []string{"dracula_appears"},
		},
		{
			name: "story event NOT triggered - variable mismatch",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						StoryEvents: map[string]StoryEvent{
							"dracula_appears": {
								When: conditionals.ConditionalWhen{
									Vars: map[string]string{"opened_grimoire": "true"},
								},
								Prompt: "Count Dracula materializes from the shadows.",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName: "castle",
				vars:      map[string]string{"opened_grimoire": "false"},
			},
			expected: nil,
		},
		{
			name: "story event triggered by scene turn counter",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						StoryEvents: map[string]StoryEvent{
							"lightning_strike": {
								When: conditionals.ConditionalWhen{
									SceneTurnCounter: intPtr(4),
								},
								Prompt: "A massive LIGHTNING bolt strikes the castle tower!",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName:        "castle",
				sceneTurnCounter: 4,
			},
			expected: []string{"lightning_strike"},
		},
		{
			name: "story event NOT triggered - turn counter too low",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						StoryEvents: map[string]StoryEvent{
							"lightning_strike": {
								When: conditionals.ConditionalWhen{
									SceneTurnCounter: intPtr(4),
								},
								Prompt: "A massive LIGHTNING bolt strikes the castle tower!",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName:        "castle",
				sceneTurnCounter: 3,
			},
			expected: nil,
		},
		{
			name: "story event triggered by multiple conditions (AND logic)",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						StoryEvents: map[string]StoryEvent{
							"complex_event": {
								When: conditionals.ConditionalWhen{
									Vars:             map[string]string{"has_key": "true", "door_locked": "true"},
									SceneTurnCounter: intPtr(3),
								},
								Prompt: "The key glows as you approach the locked door.",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName:        "castle",
				vars:             map[string]string{"has_key": "true", "door_locked": "true"},
				sceneTurnCounter: 3,
			},
			expected: []string{"complex_event"},
		},
		{
			name: "story event NOT triggered - one condition fails",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						StoryEvents: map[string]StoryEvent{
							"complex_event": {
								When: conditionals.ConditionalWhen{
									Vars:             map[string]string{"has_key": "true", "door_locked": "true"},
									SceneTurnCounter: intPtr(3),
								},
								Prompt: "The key glows as you approach the locked door.",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName:        "castle",
				vars:             map[string]string{"has_key": "true", "door_locked": "false"}, // door_locked is false
				sceneTurnCounter: 3,
			},
			expected: nil,
		},
		{
			name: "multiple story events - some triggered",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						StoryEvents: map[string]StoryEvent{
							"event1": {
								When: conditionals.ConditionalWhen{
									Vars: map[string]string{"trigger1": "true"},
								},
								Prompt: "Event 1 happens",
							},
							"event2": {
								When: conditionals.ConditionalWhen{
									Vars: map[string]string{"trigger2": "true"},
								},
								Prompt: "Event 2 happens",
							},
							"event3": {
								When: conditionals.ConditionalWhen{
									Vars: map[string]string{"trigger3": "true"},
								},
								Prompt: "Event 3 happens",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName: "castle",
				vars: map[string]string{
					"trigger1": "true",
					"trigger2": "false",
					"trigger3": "true",
				},
			},
			expected: []string{"event1", "event3"},
		},
		{
			name: "story event with turn counter",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"forest": {
						StoryEvents: map[string]StoryEvent{
							"wolf_attack": {
								When: conditionals.ConditionalWhen{
									TurnCounter: intPtr(10),
								},
								Prompt: "Wolves emerge from the shadows!",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName:   "forest",
				turnCounter: 10,
			},
			expected: []string{"wolf_attack"},
		},
		{
			name: "story event with min scene turns",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"dungeon": {
						StoryEvents: map[string]StoryEvent{
							"water_rising": {
								When: conditionals.ConditionalWhen{
									MinSceneTurns: intPtr(5),
								},
								Prompt: "The water level is rising dangerously high!",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName:        "dungeon",
				sceneTurnCounter: 7,
			},
			expected: []string{"water_rising"},
		},
		{
			name: "story event with min turns",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"spaceship": {
						StoryEvents: map[string]StoryEvent{
							"oxygen_warning": {
								When: conditionals.ConditionalWhen{
									MinTurns: intPtr(15),
								},
								Prompt: "WARNING: Oxygen levels critical!",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName:   "spaceship",
				turnCounter: 20,
			},
			expected: []string{"oxygen_warning"},
		},
		{
			name: "story event with location condition",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"town": {
						StoryEvents: map[string]StoryEvent{
							"merchant_appears": {
								When: conditionals.ConditionalWhen{
									Location: "market_square",
								},
								Prompt: "A mysterious merchant approaches you.",
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName:    "town",
				userLocation: "market_square",
			},
			expected: []string{"merchant_appears"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scenario.EvaluateStoryEvents(tt.gsView)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d triggered events, got %d", len(tt.expected), len(result))
				t.Errorf("Expected: %v", tt.expected)
				var gotKeys []string
				for key := range result {
					gotKeys = append(gotKeys, key)
				}
				t.Errorf("Got: %v", gotKeys)
				return
			}

			for _, expectedKey := range tt.expected {
				if _, exists := result[expectedKey]; !exists {
					t.Errorf("Expected event key %q not found in results", expectedKey)
				}
			}
		})
	}
}
