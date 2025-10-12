package scenario

import (
	"testing"
)

// mockGameStateView implements GameStateView for testing
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
		prompts  []ContingencyPrompt
		gsView   GameStateView
		expected []string
	}{
		{
			name:     "no prompts",
			prompts:  []ContingencyPrompt{},
			gsView:   &mockGameStateView{},
			expected: []string{},
		},
		{
			name: "prompt without condition",
			prompts: []ContingencyPrompt{
				{Prompt: "Always show this"},
			},
			gsView:   &mockGameStateView{},
			expected: []string{"Always show this"},
		},
		{
			name: "multiple prompts without conditions",
			prompts: []ContingencyPrompt{
				{Prompt: "Prompt 1"},
				{Prompt: "Prompt 2"},
				{Prompt: "Prompt 3"},
			},
			gsView:   &mockGameStateView{},
			expected: []string{"Prompt 1", "Prompt 2", "Prompt 3"},
		},
		{
			name: "prompt with satisfied var condition",
			prompts: []ContingencyPrompt{
				{
					Prompt: "Show when has_key is true",
					When: &ConditionalWhen{
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
			prompts: []ContingencyPrompt{
				{
					Prompt: "Show when has_key is true",
					When: &ConditionalWhen{
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
			prompts: []ContingencyPrompt{
				{Prompt: "Always show"},
				{
					Prompt: "Show when has_item is true",
					When: &ConditionalWhen{
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
			prompts: []ContingencyPrompt{
				{
					Prompt: "Show on turn 5",
					When: &ConditionalWhen{
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
			prompts: []ContingencyPrompt{
				{
					Prompt: "Show on turn 5",
					When: &ConditionalWhen{
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
			prompts: []ContingencyPrompt{
				{
					Prompt: "Show after 3 scene turns",
					When: &ConditionalWhen{
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
			prompts: []ContingencyPrompt{
				{
					Prompt: "Show after 3 scene turns",
					When: &ConditionalWhen{
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
			prompts: []ContingencyPrompt{
				{
					Prompt: "Show when has_sword",
					When: &ConditionalWhen{
						Vars: map[string]string{"has_sword": "true"},
					},
				},
				{
					Prompt: "Show when has_shield",
					When: &ConditionalWhen{
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
		gsView   GameStateView
		expected []string // Just the event names for simplicity
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
						StoryEvents: []StoryEvent{},
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
						StoryEvents: []StoryEvent{
							{
								Name: "dracula_appears",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "dracula_appears",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "lightning_strike",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "lightning_strike",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "complex_event",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "complex_event",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "event1",
								When: ConditionalWhen{
									Vars: map[string]string{"trigger1": "true"},
								},
								Prompt: "Event 1 happens",
							},
							{
								Name: "event2",
								When: ConditionalWhen{
									Vars: map[string]string{"trigger2": "true"},
								},
								Prompt: "Event 2 happens",
							},
							{
								Name: "event3",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "wolf_attack",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "water_rising",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "oxygen_warning",
								When: ConditionalWhen{
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
						StoryEvents: []StoryEvent{
							{
								Name: "merchant_appears",
								When: ConditionalWhen{
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
				var gotNames []string
				for _, e := range result {
					gotNames = append(gotNames, e.Name)
				}
				t.Errorf("Got: %v", gotNames)
				return
			}

			for i, expectedName := range tt.expected {
				if result[i].Name != expectedName {
					t.Errorf("Event %d: expected name %q, got %q", i, expectedName, result[i].Name)
				}
			}
		})
	}
}
