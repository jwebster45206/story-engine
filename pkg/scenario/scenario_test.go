package scenario

import (
	"encoding/json"
	"testing"
)

func TestScene_UnmarshalStoryEvents(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
		validate    func(*testing.T, Scene)
	}{
		{
			name: "scene with single story event",
			jsonData: `{
				"story": "Test scene",
				"story_events": [
					{
						"name": "test_event",
						"when": {
							"vars": {"test_var": "true"}
						},
						"prompt": "Test event triggered."
					}
				]
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 1 {
					t.Errorf("Expected 1 story event, got %d", len(scene.StoryEvents))
					return
				}
				event := scene.StoryEvents[0]
				if event.Name != "test_event" {
					t.Errorf("Expected event name 'test_event', got %q", event.Name)
				}
				if event.Prompt != "Test event triggered." {
					t.Errorf("Expected prompt 'Test event triggered.', got %q", event.Prompt)
				}
				if len(event.When.Vars) != 1 {
					t.Errorf("Expected 1 var condition, got %d", len(event.When.Vars))
				}
				if event.When.Vars["test_var"] != "true" {
					t.Errorf("Expected test_var='true', got %q", event.When.Vars["test_var"])
				}
			},
		},
		{
			name: "scene with multiple story events",
			jsonData: `{
				"story": "Test scene",
				"story_events": [
					{
						"name": "event1",
						"when": {
							"vars": {"var1": "true"}
						},
						"prompt": "First event."
					},
					{
						"name": "event2",
						"when": {
							"scene_turn_counter": 5
						},
						"prompt": "Second event."
					},
					{
						"name": "event3",
						"when": {
							"turn_counter": 10
						},
						"prompt": "Third event."
					}
				]
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 3 {
					t.Errorf("Expected 3 story events, got %d", len(scene.StoryEvents))
					return
				}

				// Validate first event
				if scene.StoryEvents[0].Name != "event1" {
					t.Errorf("Expected first event name 'event1', got %q", scene.StoryEvents[0].Name)
				}

				// Validate second event
				if scene.StoryEvents[1].Name != "event2" {
					t.Errorf("Expected second event name 'event2', got %q", scene.StoryEvents[1].Name)
				}
				if scene.StoryEvents[1].When.SceneTurnCounter == nil {
					t.Error("Expected scene_turn_counter condition, got nil")
				} else if *scene.StoryEvents[1].When.SceneTurnCounter != 5 {
					t.Errorf("Expected scene_turn_counter=5, got %d", *scene.StoryEvents[1].When.SceneTurnCounter)
				}

				// Validate third event
				if scene.StoryEvents[2].Name != "event3" {
					t.Errorf("Expected third event name 'event3', got %q", scene.StoryEvents[2].Name)
				}
				if scene.StoryEvents[2].When.TurnCounter == nil {
					t.Error("Expected turn_counter condition, got nil")
				} else if *scene.StoryEvents[2].When.TurnCounter != 10 {
					t.Errorf("Expected turn_counter=10, got %d", *scene.StoryEvents[2].When.TurnCounter)
				}
			},
		},
		{
			name: "scene with story event with multiple conditions",
			jsonData: `{
				"story": "Test scene",
				"story_events": [
					{
						"name": "complex_event",
						"when": {
							"vars": {"has_key": "true", "door_locked": "true"},
							"scene_turn_counter": 3,
							"location": "dungeon"
						},
						"prompt": "Complex event triggered."
					}
				]
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 1 {
					t.Errorf("Expected 1 story event, got %d", len(scene.StoryEvents))
					return
				}

				event := scene.StoryEvents[0]
				if len(event.When.Vars) != 2 {
					t.Errorf("Expected 2 var conditions, got %d", len(event.When.Vars))
				}
				if event.When.SceneTurnCounter == nil {
					t.Error("Expected scene_turn_counter condition, got nil")
				}
				if event.When.Location != "dungeon" {
					t.Errorf("Expected location='dungeon', got %q", event.When.Location)
				}
			},
		},
		{
			name: "scene with no story events",
			jsonData: `{
				"story": "Test scene",
				"story_events": []
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 0 {
					t.Errorf("Expected 0 story events, got %d", len(scene.StoryEvents))
				}
			},
		},
		{
			name: "scene without story_events field",
			jsonData: `{
				"story": "Test scene"
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if scene.StoryEvents != nil && len(scene.StoryEvents) != 0 {
					t.Errorf("Expected nil or empty story events, got %d", len(scene.StoryEvents))
				}
			},
		},
		{
			name: "story event with min_scene_turns",
			jsonData: `{
				"story": "Test scene",
				"story_events": [
					{
						"name": "min_turns_event",
						"when": {
							"min_scene_turns": 5
						},
						"prompt": "Minimum turns reached."
					}
				]
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 1 {
					t.Errorf("Expected 1 story event, got %d", len(scene.StoryEvents))
					return
				}
				event := scene.StoryEvents[0]
				if event.When.MinSceneTurns == nil {
					t.Error("Expected min_scene_turns condition, got nil")
				} else if *event.When.MinSceneTurns != 5 {
					t.Errorf("Expected min_scene_turns=5, got %d", *event.When.MinSceneTurns)
				}
			},
		},
		{
			name: "story event with min_turns",
			jsonData: `{
				"story": "Test scene",
				"story_events": [
					{
						"name": "global_min_turns",
						"when": {
							"min_turns": 20
						},
						"prompt": "Global minimum turns reached."
					}
				]
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 1 {
					t.Errorf("Expected 1 story event, got %d", len(scene.StoryEvents))
					return
				}
				event := scene.StoryEvents[0]
				if event.When.MinTurns == nil {
					t.Error("Expected min_turns condition, got nil")
				} else if *event.When.MinTurns != 20 {
					t.Errorf("Expected min_turns=20, got %d", *event.When.MinTurns)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var scene Scene
			err := json.Unmarshal([]byte(tt.jsonData), &scene)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, scene)
			}
		})
	}
}

func TestScenario_UnmarshalStoryEvents(t *testing.T) {
	jsonData := `{
		"name": "Test Scenario",
		"story": "A test scenario with story events",
		"opening_scene": "intro",
		"scenes": {
			"intro": {
				"story": "Introduction scene",
				"story_events": [
					{
						"name": "intro_event",
						"when": {
							"vars": {"started": "true"}
						},
						"prompt": "The adventure begins!"
					}
				]
			},
			"castle": {
				"story": "Castle scene",
				"story_events": [
					{
						"name": "dracula_appears",
						"when": {
							"vars": {"opened_grimoire": "true"}
						},
						"prompt": "Count Dracula materializes from the shadows."
					},
					{
						"name": "lightning_strike",
						"when": {
							"scene_turn_counter": 4
						},
						"prompt": "A massive LIGHTNING bolt strikes the castle tower!"
					}
				]
			}
		}
	}`

	var scenario Scenario
	err := json.Unmarshal([]byte(jsonData), &scenario)
	if err != nil {
		t.Fatalf("Failed to unmarshal scenario: %v", err)
	}

	// Verify intro scene
	introScene, ok := scenario.Scenes["intro"]
	if !ok {
		t.Fatal("Expected 'intro' scene to exist")
	}
	if len(introScene.StoryEvents) != 1 {
		t.Errorf("Expected 1 story event in intro scene, got %d", len(introScene.StoryEvents))
	}
	if introScene.StoryEvents[0].Name != "intro_event" {
		t.Errorf("Expected intro event name 'intro_event', got %q", introScene.StoryEvents[0].Name)
	}

	// Verify castle scene
	castleScene, ok := scenario.Scenes["castle"]
	if !ok {
		t.Fatal("Expected 'castle' scene to exist")
	}
	if len(castleScene.StoryEvents) != 2 {
		t.Errorf("Expected 2 story events in castle scene, got %d", len(castleScene.StoryEvents))
	}
	if castleScene.StoryEvents[0].Name != "dracula_appears" {
		t.Errorf("Expected first event name 'dracula_appears', got %q", castleScene.StoryEvents[0].Name)
	}
	if castleScene.StoryEvents[1].Name != "lightning_strike" {
		t.Errorf("Expected second event name 'lightning_strike', got %q", castleScene.StoryEvents[1].Name)
	}
}

func TestStoryEvent_MarshalUnmarshal(t *testing.T) {
	original := StoryEvent{
		Name: "test_event",
		When: ConditionalWhen{
			Vars:             map[string]string{"test": "true", "other": "false"},
			SceneTurnCounter: intPtr(5),
			Location:         "dungeon",
		},
		Prompt: "Test event prompt with special characters: \"quotes\", \nnewlines",
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal StoryEvent: %v", err)
	}

	// Unmarshal back
	var restored StoryEvent
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal StoryEvent: %v", err)
	}

	// Verify all fields
	if restored.Name != original.Name {
		t.Errorf("Name: expected %q, got %q", original.Name, restored.Name)
	}
	if restored.Prompt != original.Prompt {
		t.Errorf("Prompt: expected %q, got %q", original.Prompt, restored.Prompt)
	}
	if len(restored.When.Vars) != len(original.When.Vars) {
		t.Errorf("Vars length: expected %d, got %d", len(original.When.Vars), len(restored.When.Vars))
	}
	for k, v := range original.When.Vars {
		if restored.When.Vars[k] != v {
			t.Errorf("Var %q: expected %q, got %q", k, v, restored.When.Vars[k])
		}
	}
	if restored.When.SceneTurnCounter == nil || *restored.When.SceneTurnCounter != *original.When.SceneTurnCounter {
		t.Errorf("SceneTurnCounter mismatch")
	}
	if restored.When.Location != original.When.Location {
		t.Errorf("Location: expected %q, got %q", original.When.Location, restored.When.Location)
	}
}
