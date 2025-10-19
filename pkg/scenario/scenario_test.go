package scenario

import (
	"encoding/json"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
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
				"story_events": {
					"test_event": {
						"when": {
							"vars": {"test_var": "true"}
						},
						"prompt": "Test event triggered."
					}
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 1 {
					t.Errorf("Expected 1 story event, got %d", len(scene.StoryEvents))
					return
				}
				event, exists := scene.StoryEvents["test_event"]
				if !exists {
					t.Error("Expected 'test_event' key to exist")
					return
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
				"story_events": {
					"event1": {
						"when": {
							"vars": {"var1": "true"}
						},
						"prompt": "First event."
					},
					"event2": {
						"when": {
							"scene_turn_counter": 5
						},
						"prompt": "Second event."
					},
					"event3": {
						"when": {
							"turn_counter": 10
						},
						"prompt": "Third event."
					}
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 3 {
					t.Errorf("Expected 3 story events, got %d", len(scene.StoryEvents))
					return
				}

				// Validate first event
				event1, exists := scene.StoryEvents["event1"]
				if !exists {
					t.Error("Expected 'event1' key to exist")
					return
				}
				if event1.Prompt != "First event." {
					t.Errorf("Expected prompt 'First event.', got %q", event1.Prompt)
				}

				// Validate second event
				event2, exists := scene.StoryEvents["event2"]
				if !exists {
					t.Error("Expected 'event2' key to exist")
					return
				}
				if event2.When.SceneTurnCounter == nil {
					t.Error("Expected scene_turn_counter condition, got nil")
				} else if *event2.When.SceneTurnCounter != 5 {
					t.Errorf("Expected scene_turn_counter=5, got %d", *event2.When.SceneTurnCounter)
				}

				// Validate third event
				event3, exists := scene.StoryEvents["event3"]
				if !exists {
					t.Error("Expected 'event3' key to exist")
					return
				}
				if event3.When.TurnCounter == nil {
					t.Error("Expected turn_counter condition, got nil")
				} else if *event3.When.TurnCounter != 10 {
					t.Errorf("Expected turn_counter=10, got %d", *event3.When.TurnCounter)
				}
			},
		},
		{
			name: "scene with story event with multiple conditions",
			jsonData: `{
				"story": "Test scene",
				"story_events": {
					"complex_event": {
						"when": {
							"vars": {"has_key": "true", "door_locked": "true"},
							"scene_turn_counter": 3,
							"location": "dungeon"
						},
						"prompt": "Complex event triggered."
					}
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 1 {
					t.Errorf("Expected 1 story event, got %d", len(scene.StoryEvents))
					return
				}

				event, exists := scene.StoryEvents["complex_event"]
				if !exists {
					t.Error("Expected 'complex_event' key to exist")
					return
				}
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
				"story_events": {}
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
				if len(scene.StoryEvents) != 0 {
					t.Errorf("Expected nil or empty story events, got %d", len(scene.StoryEvents))
				}
			},
		},
		{
			name: "story event with min_scene_turns",
			jsonData: `{
				"story": "Test scene",
				"story_events": {
					"min_turns_event": {
						"when": {
							"min_scene_turns": 5
						},
						"prompt": "Minimum turns reached."
					}
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 1 {
					t.Errorf("Expected 1 story event, got %d", len(scene.StoryEvents))
					return
				}
				event, exists := scene.StoryEvents["min_turns_event"]
				if !exists {
					t.Error("Expected 'min_turns_event' key to exist")
					return
				}
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
				"story_events": {
					"global_min_turns": {
						"when": {
							"min_turns": 20
						},
						"prompt": "Global minimum turns reached."
					}
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.StoryEvents) != 1 {
					t.Errorf("Expected 1 story event, got %d", len(scene.StoryEvents))
					return
				}
				event, exists := scene.StoryEvents["global_min_turns"]
				if !exists {
					t.Error("Expected 'global_min_turns' key to exist")
					return
				}
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
				"story_events": {
					"intro_event": {
						"when": {
							"vars": {"started": "true"}
						},
						"prompt": "The adventure begins!"
					}
				}
			},
			"castle": {
				"story": "Castle scene",
				"story_events": {
					"dracula_appears": {
						"when": {
							"vars": {"opened_grimoire": "true"}
						},
						"prompt": "Count Dracula materializes from the shadows."
					},
					"lightning_strike": {
						"when": {
							"scene_turn_counter": 4
						},
						"prompt": "A massive LIGHTNING bolt strikes the castle tower!"
					}
				}
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
	if _, exists := introScene.StoryEvents["intro_event"]; !exists {
		t.Error("Expected 'intro_event' key to exist in intro scene")
	}

	// Verify castle scene
	castleScene, ok := scenario.Scenes["castle"]
	if !ok {
		t.Fatal("Expected 'castle' scene to exist")
	}
	if len(castleScene.StoryEvents) != 2 {
		t.Errorf("Expected 2 story events in castle scene, got %d", len(castleScene.StoryEvents))
	}
	if _, exists := castleScene.StoryEvents["dracula_appears"]; !exists {
		t.Error("Expected 'dracula_appears' key to exist in castle scene")
	}
	if _, exists := castleScene.StoryEvents["lightning_strike"]; !exists {
		t.Error("Expected 'lightning_strike' key to exist in castle scene")
	}
}

func TestStoryEvent_MarshalUnmarshal(t *testing.T) {
	original := StoryEvent{
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

func TestGetLocation(t *testing.T) {
	// Create a test scenario with various locations
	scenario := &Scenario{
		Locations: map[string]Location{
			"black_pearl": {
				Name:        "Black Pearl",
				Description: "A legendary pirate ship",
			},
			"captains_cabin": {
				Name:        "Captain's Cabin",
				Description: "The captain's private quarters",
			},
			"sleepy_mermaid": {
				Name:        "Sleepy Mermaid",
				Description: "A rowdy pirate tavern",
			},
			"tortuga_market": {
				Name:        "Tortuga Market",
				Description: "A bustling marketplace",
			},
		},
	}

	tests := []struct {
		name       string
		input      string
		expectKey  string
		expectFind bool
	}{
		// Exact key matches (case-insensitive)
		{
			name:       "exact key match - black_pearl",
			input:      "black_pearl",
			expectKey:  "black_pearl",
			expectFind: true,
		},
		{
			name:       "exact key match - sleepy_mermaid",
			input:      "sleepy_mermaid",
			expectKey:  "sleepy_mermaid",
			expectFind: true,
		},

		// Case variations of keys
		{
			name:       "case insensitive key - Black_Pearl",
			input:      "Black_Pearl",
			expectKey:  "black_pearl",
			expectFind: true,
		},
		{
			name:       "case insensitive key - SLEEPY_MERMAID",
			input:      "SLEEPY_MERMAID",
			expectKey:  "sleepy_mermaid",
			expectFind: true,
		},
		{
			name:       "case insensitive key - Captains_Cabin",
			input:      "Captains_Cabin",
			expectKey:  "captains_cabin",
			expectFind: true,
		},

		// Name matches (case-insensitive)
		{
			name:       "name match - Black Pearl",
			input:      "Black Pearl",
			expectKey:  "black_pearl",
			expectFind: true,
		},
		{
			name:       "name match - Captain's Cabin",
			input:      "Captain's Cabin",
			expectKey:  "captains_cabin",
			expectFind: true,
		},
		{
			name:       "name match - Sleepy Mermaid",
			input:      "Sleepy Mermaid",
			expectKey:  "sleepy_mermaid",
			expectFind: true,
		},

		// Case variations of names
		{
			name:       "case insensitive name - black pearl",
			input:      "black pearl",
			expectKey:  "black_pearl",
			expectFind: true,
		},
		{
			name:       "case insensitive name - BLACK PEARL",
			input:      "BLACK PEARL",
			expectKey:  "black_pearl",
			expectFind: true,
		},
		{
			name:       "case insensitive name - captain's cabin",
			input:      "captain's cabin",
			expectKey:  "captains_cabin",
			expectFind: true,
		},
		{
			name:       "case insensitive name - sleepy mermaid",
			input:      "sleepy mermaid",
			expectKey:  "sleepy_mermaid",
			expectFind: true,
		},
		{
			name:       "case insensitive name - TORTUGA MARKET",
			input:      "TORTUGA MARKET",
			expectKey:  "tortuga_market",
			expectFind: true,
		},

		// Whitespace handling
		{
			name:       "trimmed input - spaces around key",
			input:      "  black_pearl  ",
			expectKey:  "black_pearl",
			expectFind: true,
		},
		{
			name:       "trimmed input - spaces around name",
			input:      "  Black Pearl  ",
			expectKey:  "black_pearl",
			expectFind: true,
		},

		// Not found cases
		{
			name:       "not found - non-existent location",
			input:      "british_docks",
			expectKey:  "",
			expectFind: false,
		},
		{
			name:       "not found - partial match shouldn't work",
			input:      "Pearl",
			expectKey:  "",
			expectFind: false,
		},
		{
			name:       "not found - empty string",
			input:      "",
			expectKey:  "",
			expectFind: false,
		},
		{
			name:       "not found - whitespace only",
			input:      "   ",
			expectKey:  "",
			expectFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, found := scenario.GetLocation(tt.input)

			if found != tt.expectFind {
				t.Errorf("Expected found=%v, got found=%v", tt.expectFind, found)
			}

			if key != tt.expectKey {
				t.Errorf("Expected key=%q, got key=%q", tt.expectKey, key)
			}
		})
	}
}

func TestGetLocation_EmptyScenario(t *testing.T) {
	scenario := &Scenario{
		Locations: map[string]Location{},
	}

	key, found := scenario.GetLocation("anywhere")
	if found {
		t.Errorf("Expected not to find location in empty scenario, but found key=%q", key)
	}
	if key != "" {
		t.Errorf("Expected empty key when not found, got %q", key)
	}
}

func TestGetLocation_NilLocations(t *testing.T) {
	scenario := &Scenario{
		Locations: nil,
	}

	key, found := scenario.GetLocation("anywhere")
	if found {
		t.Errorf("Expected not to find location with nil map, but found key=%q", key)
	}
	if key != "" {
		t.Errorf("Expected empty key when not found, got %q", key)
	}
}

func TestGetNPC(t *testing.T) {
	// Create a test scenario with various NPCs
	scenario := &Scenario{
		NPCs: map[string]actor.NPC{
			"gibbs": {
				Name:        "Gibbs",
				Type:        "pirate",
				Disposition: "loyal",
			},
			"calypso": {
				Name:        "Calypso",
				Type:        "bartender",
				Disposition: "friendly but mysterious",
			},
			"charming_danny": {
				Name:        "Charming Danny",
				Type:        "merchant",
				Disposition: "shrewd and cunning",
			},
			"shipwright": {
				Name:        "Shipwright",
				Type:        "shipwright",
				Disposition: "gruff but helpful",
			},
		},
	}

	tests := []struct {
		name       string
		input      string
		expectKey  string
		expectFind bool
	}{
		// Exact key matches (case-insensitive)
		{
			name:       "exact key match - gibbs",
			input:      "gibbs",
			expectKey:  "gibbs",
			expectFind: true,
		},
		{
			name:       "exact key match - calypso",
			input:      "calypso",
			expectKey:  "calypso",
			expectFind: true,
		},
		{
			name:       "exact key match - charming_danny",
			input:      "charming_danny",
			expectKey:  "charming_danny",
			expectFind: true,
		},

		// Case variations of keys
		{
			name:       "case insensitive key - Gibbs",
			input:      "Gibbs",
			expectKey:  "gibbs",
			expectFind: true,
		},
		{
			name:       "case insensitive key - CALYPSO",
			input:      "CALYPSO",
			expectKey:  "calypso",
			expectFind: true,
		},
		{
			name:       "case insensitive key - Charming_Danny",
			input:      "Charming_Danny",
			expectKey:  "charming_danny",
			expectFind: true,
		},
		{
			name:       "case insensitive key - SHIPWRIGHT",
			input:      "SHIPWRIGHT",
			expectKey:  "shipwright",
			expectFind: true,
		},

		// Name matches (case-insensitive)
		{
			name:       "name match - Gibbs",
			input:      "Gibbs",
			expectKey:  "gibbs",
			expectFind: true,
		},
		{
			name:       "name match - Calypso",
			input:      "Calypso",
			expectKey:  "calypso",
			expectFind: true,
		},
		{
			name:       "name match - Charming Danny",
			input:      "Charming Danny",
			expectKey:  "charming_danny",
			expectFind: true,
		},
		{
			name:       "name match - Shipwright",
			input:      "Shipwright",
			expectKey:  "shipwright",
			expectFind: true,
		},

		// Case variations of names
		{
			name:       "case insensitive name - gibbs",
			input:      "gibbs",
			expectKey:  "gibbs",
			expectFind: true,
		},
		{
			name:       "case insensitive name - GIBBS",
			input:      "GIBBS",
			expectKey:  "gibbs",
			expectFind: true,
		},
		{
			name:       "case insensitive name - calypso",
			input:      "calypso",
			expectKey:  "calypso",
			expectFind: true,
		},
		{
			name:       "case insensitive name - charming danny",
			input:      "charming danny",
			expectKey:  "charming_danny",
			expectFind: true,
		},
		{
			name:       "case insensitive name - CHARMING DANNY",
			input:      "CHARMING DANNY",
			expectKey:  "charming_danny",
			expectFind: true,
		},
		{
			name:       "case insensitive name - shipwright",
			input:      "shipwright",
			expectKey:  "shipwright",
			expectFind: true,
		},

		// Whitespace handling
		{
			name:       "trimmed input - spaces around key",
			input:      "  gibbs  ",
			expectKey:  "gibbs",
			expectFind: true,
		},
		{
			name:       "trimmed input - spaces around name",
			input:      "  Charming Danny  ",
			expectKey:  "charming_danny",
			expectFind: true,
		},

		// Not found cases
		{
			name:       "not found - non-existent NPC",
			input:      "captain_morgan",
			expectKey:  "",
			expectFind: false,
		},
		{
			name:       "not found - partial match shouldn't work",
			input:      "Danny",
			expectKey:  "",
			expectFind: false,
		},
		{
			name:       "not found - empty string",
			input:      "",
			expectKey:  "",
			expectFind: false,
		},
		{
			name:       "not found - whitespace only",
			input:      "   ",
			expectKey:  "",
			expectFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, found := scenario.GetNPC(tt.input)

			if found != tt.expectFind {
				t.Errorf("Expected found=%v, got found=%v", tt.expectFind, found)
			}

			if key != tt.expectKey {
				t.Errorf("Expected key=%q, got key=%q", tt.expectKey, key)
			}
		})
	}
}

func TestGetNPC_EmptyScenario(t *testing.T) {
	scenario := &Scenario{
		NPCs: map[string]actor.NPC{},
	}

	key, found := scenario.GetNPC("anyone")
	if found {
		t.Errorf("Expected not to find NPC in empty scenario, but found key=%q", key)
	}
	if key != "" {
		t.Errorf("Expected empty key when not found, got %q", key)
	}
}

func TestGetNPC_NilNPCs(t *testing.T) {
	scenario := &Scenario{
		NPCs: nil,
	}

	key, found := scenario.GetNPC("anyone")
	if found {
		t.Errorf("Expected not to find NPC with nil map, but found key=%q", key)
	}
	if key != "" {
		t.Errorf("Expected empty key when not found, got %q", key)
	}
}
