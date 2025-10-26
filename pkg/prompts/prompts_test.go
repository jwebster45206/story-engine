package prompts

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestGetStatePrompt(t *testing.T) {
	tests := []struct {
		name        string
		gameState   *state.GameState
		scenario    *scenario.Scenario
		expected    chat.ChatMessage
		expectError bool
	}{
		{
			name: "traditional scenario without scenes",
			gameState: &state.GameState{
				Scenario:  "test.json",
				Location:  "Tortuga",
				Inventory: []string{"cutlass", "spyglass"},
				NPCs: map[string]actor.NPC{
					"Gibbs": {
						Name:        "Gibbs",
						Type:        "pirate",
						Disposition: "loyal",
						Location:    "Black Pearl",
					},
				},
				WorldLocations: map[string]scenario.Location{
					"Tortuga": {
						Name:        "Tortuga",
						Description: "A pirate port",
						Exits:       map[string]string{"east": "Black Pearl"},
					},
				},
				Vars:               map[string]string{"test_var": "true"},
				ContingencyPrompts: []string{"Test prompt"},
			},
			scenario: &scenario.Scenario{
				Name:  "Test Scenario",
				Story: "A test adventure",
			},
			expected: chat.ChatMessage{
				Role: chat.ChatRoleSystem,
				Content: `The user is roleplaying this scenario: A test adventure

The following describes the immediately surrounding world.

// -- BEGIN WORLD STATE --
CURRENT LOCATION:
Tortuga: A pirate port
Exits:

USER'S INVENTORY: 
cutlass, spyglass

// -- END WORLD STATE --

`,
			},
		},
		{
			name: "scene-based scenario with valid scene",
			gameState: &state.GameState{
				Scenario:  "pirate.json",
				SceneName: "shipwright",
				Location:  "Tortuga",
				Inventory: []string{"cutlass"},
				Vars:      map[string]string{"scene_var": "false"},
			},
			scenario: &scenario.Scenario{
				Name:  "Pirate Adventure",
				Story: "Overall pirate story",
				Scenes: map[string]scenario.Scene{
					"shipwright": {
						Story: "Find the shipwright",
						Locations: map[string]scenario.Location{
							"Tortuga": {
								Name:        "Tortuga",
								Description: "A bustling pirate port",
								Exits:       map[string]string{"east": "Black Pearl"},
							},
						},
						NPCs: map[string]actor.NPC{
							"Shipwright": {
								Name:        "Shipwright",
								Type:        "craftsman",
								Disposition: "gruff",
								Location:    "Tortuga",
							},
						},
						Vars: map[string]string{"repairs_started": "false"},
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "Scene-specific prompt"},
						},
					},
				},
			},
			expected: chat.ChatMessage{
				Role: chat.ChatRoleSystem,
				Content: `The user is roleplaying this scenario: Overall pirate story

Find the shipwright

The following describes the immediately surrounding world.

// -- BEGIN WORLD STATE --
CURRENT LOCATION:
Tortuga: A bustling pirate port
Exits:

NPCs:
Shipwright (gruff)

USER'S INVENTORY: 
cutlass

// -- END WORLD STATE --

`,
			},
		},
		{
			name: "scene-based scenario with invalid scene",
			gameState: &state.GameState{
				Scenario:  "pirate.json",
				SceneName: "nonexistent_scene",
				Location:  "Tortuga",
			},
			scenario: &scenario.Scenario{
				Name:   "Pirate Adventure",
				Story:  "Overall pirate story",
				Scenes: map[string]scenario.Scene{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load scene if needed
			if tt.gameState.SceneName != "" {
				err := tt.gameState.LoadScene(tt.scenario, tt.gameState.SceneName)
				if err != nil {
					if !tt.expectError {
						t.Fatalf("Unexpected error loading scene: %v", err)
					}
					return
				}
			}

			result, err := GetStatePrompt(tt.gameState, tt.scenario)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Compare role
			if result.Role != tt.expected.Role {
				t.Errorf("Expected role %s, got %s", tt.expected.Role, result.Role)
			}

			// Compare content directly
			if result.Content != tt.expected.Content {
				t.Errorf("Expected content:\n%s\n\nGot content:\n%s", tt.expected.Content, result.Content)
			}
		})
	}
}
