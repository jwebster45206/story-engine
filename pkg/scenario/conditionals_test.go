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
