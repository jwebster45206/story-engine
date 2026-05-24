package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestGetStatePrompt(t *testing.T) {
	type check struct {
		mustContain    []string
		mustNotContain []string
	}
	tests := []struct {
		name        string
		gameState   *state.GameState
		scenario    *scenario.Scenario
		check       check
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
			check: check{
				mustContain: []string{
					"The user is roleplaying this scenario: A test adventure",
					"<world_state>",
					"<just_entered>false</just_entered>",
					"<current_location>",
					"Tortuga",
					"A pirate port",
					"Exits (the ONLY directions reachable this turn):",
					"- east -> Black Pearl",
					"<user_inventory>",
					"cutlass, spyglass",
					"</world_state>",
				},
				mustNotContain: []string{
					"// -- BEGIN WORLD STATE --",
					"// -- END WORLD STATE --",
				},
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
			check: check{
				mustContain: []string{
					"The user is roleplaying this scenario: Overall pirate story",
					"Find the shipwright",
					"<world_state>",
					"<current_location>",
					"Tortuga",
					"A bustling pirate port",
					"NPCs here: Shipwright",
					"Exits (the ONLY directions reachable this turn):",
					"<user_inventory>",
					"cutlass",
					"<world_state_rules>",
					"Movement: the player may only choose one of:",
				},
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

			if result.Role != chat.ChatRoleSystem {
				t.Errorf("Expected role %s, got %s", chat.ChatRoleSystem, result.Role)
			}

			for _, want := range tt.check.mustContain {
				if !strings.Contains(result.Content, want) {
					t.Errorf("expected content to contain %q\n--- got ---\n%s", want, result.Content)
				}
			}
			for _, unwanted := range tt.check.mustNotContain {
				if strings.Contains(result.Content, unwanted) {
					t.Errorf("expected content to NOT contain %q\n--- got ---\n%s", unwanted, result.Content)
				}
			}
		})
	}
}
