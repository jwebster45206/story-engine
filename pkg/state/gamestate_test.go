package state

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestGameState_GetStatePrompt(t *testing.T) {
	tests := []struct {
		name        string
		gameState   *GameState
		scenario    *scenario.Scenario
		expectError bool
		description string
	}{
		{
			name:        "nil gamestate",
			gameState:   nil,
			scenario:    &scenario.Scenario{},
			expectError: true,
			description: "should return error when gamestate is nil",
		},
		{
			name: "traditional scenario without scenes",
			gameState: &GameState{
				ID:        uuid.New(),
				Scenario:  "test.json",
				Location:  "Tortuga",
				Inventory: []string{"cutlass", "spyglass"},
				NPCs: map[string]scenario.NPC{
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
			expectError: false,
			description: "should handle traditional scenario without scenes",
		},
		{
			name: "scene-based scenario with valid scene",
			gameState: &GameState{
				ID:        uuid.New(),
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
						NPCs: map[string]scenario.NPC{
							"Shipwright": {
								Name:        "Shipwright",
								Type:        "craftsman",
								Disposition: "gruff",
								Location:    "Tortuga",
							},
						},
						Vars:               map[string]string{"repairs_started": "false"},
						ContingencyPrompts: []string{"Scene-specific prompt"},
					},
				},
			},
			expectError: false,
			description: "should handle scene-based scenario correctly",
		},
		{
			name: "scene-based scenario with invalid scene",
			gameState: &GameState{
				ID:        uuid.New(),
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
			description: "should return error when scene not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.gameState.GetStatePrompt(tt.scenario)

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

			// Validate the result structure
			if result.Role != chat.ChatRoleSystem {
				t.Errorf("Expected role %s, got %s", chat.ChatRoleSystem, result.Role)
			}

			if result.Content == "" {
				t.Errorf("Expected non-empty content")
			}

			// For traditional scenarios, verify it uses the scenario story
			if tt.gameState != nil && tt.gameState.SceneName == "" {
				if !strings.Contains(result.Content, tt.scenario.Story) {
					t.Errorf("Expected content to contain scenario story '%s'", tt.scenario.Story)
				}
			}

			// For scene-based scenarios, verify it uses the scene story
			if tt.gameState != nil && tt.gameState.SceneName != "" {
				scene := tt.scenario.Scenes[tt.gameState.SceneName]
				expectedStory := scene.Story
				if expectedStory == "" {
					expectedStory = tt.scenario.Story // fallback
				}
				if !strings.Contains(result.Content, expectedStory) {
					t.Errorf("Expected content to contain story '%s'", expectedStory)
				}
			}

			// Verify JSON is present in the content
			if !strings.Contains(result.Content, "```json") {
				t.Errorf("Expected content to contain JSON block")
			}
		})
	}
}

func TestGameState_GetScenePrompt(t *testing.T) {
	tests := []struct {
		name        string
		gameState   *GameState
		scenario    *scenario.Scenario
		scene       *scenario.Scene
		expectError bool
		description string
	}{
		{
			name:        "nil gamestate",
			gameState:   nil,
			scenario:    &scenario.Scenario{},
			scene:       &scenario.Scene{},
			expectError: true,
			description: "should return error when gamestate is nil",
		},
		{
			name:        "nil scene",
			gameState:   &GameState{},
			scenario:    &scenario.Scenario{},
			scene:       nil,
			expectError: true,
			description: "should return error when scene is nil",
		},
		{
			name: "valid scene with story",
			gameState: &GameState{
				ID:                 uuid.New(),
				Location:           "Tortuga",
				Inventory:          []string{"cutlass", "lockpicks"},
				Vars:               map[string]string{"scene_var": "true"},
				ContingencyPrompts: []string{"Global prompt"},
			},
			scenario: &scenario.Scenario{
				Name:  "Pirate Adventure",
				Story: "Main pirate story",
			},
			scene: &scenario.Scene{
				Story: "Scene-specific story about finding the shipwright",
				Locations: map[string]scenario.Location{
					"Tortuga": {
						Name:        "Tortuga",
						Description: "A bustling port",
						Exits:       map[string]string{"east": "Ship"},
					},
				},
				NPCs: map[string]scenario.NPC{
					"Shipwright": {
						Name:        "Shipwright",
						Type:        "craftsman",
						Disposition: "helpful",
						Location:    "Tortuga",
					},
				},
				Vars:               map[string]string{"repairs_needed": "true"},
				ContingencyPrompts: []string{"Scene prompt"},
			},
			expectError: false,
			description: "should handle valid scene with all components",
		},
		{
			name: "scene without story falls back to scenario story",
			gameState: &GameState{
				ID:       uuid.New(),
				Location: "Tortuga",
			},
			scenario: &scenario.Scenario{
				Name:  "Pirate Adventure",
				Story: "Main pirate story",
			},
			scene: &scenario.Scene{
				Story: "", // Empty story should fallback to scenario story
				Locations: map[string]scenario.Location{
					"Tortuga": {
						Name: "Tortuga",
					},
				},
			},
			expectError: false,
			description: "should fallback to scenario story when scene story is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.gameState.GetScenePrompt(tt.scenario, tt.scene)

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

			// Validate the result structure
			if result.Role != chat.ChatRoleSystem {
				t.Errorf("Expected role %s, got %s", chat.ChatRoleSystem, result.Role)
			}

			if result.Content == "" {
				t.Errorf("Expected non-empty content")
			}

			// Check that the correct story is used
			expectedStory := tt.scene.Story
			if expectedStory == "" {
				expectedStory = tt.scenario.Story
			}
			if !strings.Contains(result.Content, expectedStory) {
				t.Errorf("Expected content to contain story '%s', got: %s", expectedStory, result.Content)
			}

			// Verify JSON structure is present
			if !strings.Contains(result.Content, "```json") {
				t.Errorf("Expected content to contain JSON block")
			}

			// Parse and validate the JSON structure
			jsonStart := strings.Index(result.Content, "```json\n") + 8
			jsonEnd := strings.Index(result.Content[jsonStart:], "\n```")
			if jsonEnd == -1 {
				t.Errorf("Could not find end of JSON block")
				return
			}
			jsonContent := result.Content[jsonStart : jsonStart+jsonEnd]

			var promptState PromptState
			if err := json.Unmarshal([]byte(jsonContent), &promptState); err != nil {
				t.Errorf("Failed to parse JSON in prompt: %v\nJSON: %s", err, jsonContent)
				return
			}

			// Validate that scene data is used instead of gamestate data
			if tt.scene != nil {
				// Check that scene NPCs are used
				for npcName := range tt.scene.NPCs {
					if _, exists := promptState.NPCs[npcName]; !exists {
						t.Errorf("Expected scene NPC '%s' to be in prompt state", npcName)
					}
				}

				// Check that scene locations are used
				for locName := range tt.scene.Locations {
					if _, exists := promptState.WorldLocations[locName]; !exists {
						t.Errorf("Expected scene location '%s' to be in prompt state", locName)
					}
				}

				// Check that contingency prompts are combined
				expectedPrompts := len(tt.gameState.ContingencyPrompts) + len(tt.scene.ContingencyPrompts)
				if len(promptState.ContingencyPrompts) != expectedPrompts {
					t.Errorf("Expected %d contingency prompts, got %d", expectedPrompts, len(promptState.ContingencyPrompts))
				}
			}
		})
	}
}

func TestGameState_GetStatePrompt_JSONStructure(t *testing.T) {
	// Test that the JSON structure in the prompt is valid and contains expected fields
	gameState := &GameState{
		ID:                 uuid.New(),
		Scenario:           "test.json",
		SceneName:          "test_scene",
		Location:           "TestLocation",
		Inventory:          []string{"item1", "item2"},
		Vars:               map[string]string{"test_var": "value"},
		ContingencyPrompts: []string{"Test contingency"},
	}

	scenario := &scenario.Scenario{
		Name:  "Test Scenario",
		Story: "Test story",
		Scenes: map[string]scenario.Scene{
			"test_scene": {
				Story: "Test scene story",
				Locations: map[string]scenario.Location{
					"TestLocation": {
						Name:        "TestLocation",
						Description: "Test location",
					},
				},
				NPCs: map[string]scenario.NPC{
					"TestNPC": {
						Name:     "TestNPC",
						Location: "TestLocation",
					},
				},
				ContingencyPrompts: []string{"Scene contingency"},
			},
		},
	}

	result, err := gameState.GetStatePrompt(scenario)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Extract and parse the JSON
	jsonStart := strings.Index(result.Content, "```json\n") + 8
	jsonEnd := strings.Index(result.Content[jsonStart:], "\n```")
	jsonContent := result.Content[jsonStart : jsonStart+jsonEnd]

	var promptState PromptState
	if err := json.Unmarshal([]byte(jsonContent), &promptState); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nJSON: %s", err, jsonContent)
	}

	// Validate structure
	if promptState.Location != gameState.Location {
		t.Errorf("Expected location %s, got %s", gameState.Location, promptState.Location)
	}

	if len(promptState.Inventory) != len(gameState.Inventory) {
		t.Errorf("Expected %d inventory items, got %d", len(gameState.Inventory), len(promptState.Inventory))
	}

	// Should use scene data, not gamestate data
	scene := scenario.Scenes["test_scene"]
	if len(promptState.NPCs) != len(scene.NPCs) {
		t.Errorf("Expected %d NPCs from scene, got %d", len(scene.NPCs), len(promptState.NPCs))
	}

	if len(promptState.WorldLocations) != len(scene.Locations) {
		t.Errorf("Expected %d locations from scene, got %d", len(scene.Locations), len(promptState.WorldLocations))
	}

	// Contingency prompts should be combined
	expectedPrompts := len(gameState.ContingencyPrompts) + len(scene.ContingencyPrompts)
	if len(promptState.ContingencyPrompts) != expectedPrompts {
		t.Errorf("Expected %d total contingency prompts, got %d", expectedPrompts, len(promptState.ContingencyPrompts))
	}
}
