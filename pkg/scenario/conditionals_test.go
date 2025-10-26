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

// TestScenario_EvaluateConditionals_WithPrompt tests conditionals with prompt field
func TestScenario_EvaluateConditionals_WithPrompt(t *testing.T) {
	storyEventPrompt := "STORY EVENT: Count Dracula materializes from the shadows."
	regularPrompt := "The room grows cold."

	tests := []struct {
		name     string
		scenario *Scenario
		gsView   conditionals.GameStateView
		expected []string // Expected conditional keys
	}{
		{
			name: "conditional with story event prompt",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						Conditionals: map[string]Conditional{
							"dracula_appears": {
								When: conditionals.ConditionalWhen{
									Vars: map[string]string{"opened_grimoire": "true"},
								},
								Then: ConditionalThen{
									Prompt: &storyEventPrompt,
								},
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
			name: "conditional with regular prompt (no STORY EVENT prefix)",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						Conditionals: map[string]Conditional{
							"cold_room": {
								When: conditionals.ConditionalWhen{
									Vars: map[string]string{"window_open": "true"},
								},
								Then: ConditionalThen{
									Prompt: &regularPrompt,
								},
							},
						},
					},
				},
			},
			gsView: &mockGameStateView{
				sceneName: "castle",
				vars:      map[string]string{"window_open": "true"},
			},
			expected: []string{"cold_room"},
		},
		{
			name: "conditional NOT triggered - variable mismatch",
			scenario: &Scenario{
				Scenes: map[string]Scene{
					"castle": {
						Conditionals: map[string]Conditional{
							"dracula_appears": {
								When: conditionals.ConditionalWhen{
									Vars: map[string]string{"opened_grimoire": "true"},
								},
								Then: ConditionalThen{
									Prompt: &storyEventPrompt,
								},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scenario.EvaluateConditionals(tt.gsView)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d triggered conditionals, got %d", len(tt.expected), len(result))
				t.Errorf("Expected: %v", tt.expected)
				var gotKeys []string
				for key := range result {
					gotKeys = append(gotKeys, key)
				}
				t.Errorf("Got: %v", gotKeys)
				return
			}

			for _, expectedKey := range tt.expected {
				if cond, exists := result[expectedKey]; !exists {
					t.Errorf("Expected conditional key %q not found in results", expectedKey)
				} else if cond.Then.Prompt == nil {
					t.Errorf("Expected conditional %q to have a prompt", expectedKey)
				}
			}
		})
	}
}
