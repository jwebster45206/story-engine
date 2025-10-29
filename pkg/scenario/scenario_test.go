package scenario

import (
	"encoding/json"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
)

// TestScene_UnmarshalConditionalPrompt tests conditionals with prompt field
func TestScene_UnmarshalConditionalPrompt(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
		validate    func(*testing.T, Scene)
	}{
		{
			name: "conditional with story event prompt",
			jsonData: `{
				"story": "Test scene",
				"conditionals": {
					"dracula_appears": {
						"when": {
							"vars": {"opened_grimoire": "true"}
						},
						"then": {
							"prompt": "STORY EVENT: Count Dracula materializes from the shadows."
						}
					}
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				if len(scene.Conditionals) != 1 {
					t.Errorf("Expected 1 conditional, got %d", len(scene.Conditionals))
					return
				}
				cond, exists := scene.Conditionals["dracula_appears"]
				if !exists {
					t.Error("Expected 'dracula_appears' key to exist")
					return
				}
				if cond.Then.Prompt == nil {
					t.Error("Expected prompt to be set")
					return
				}
				expected := "STORY EVENT: Count Dracula materializes from the shadows."
				if *cond.Then.Prompt != expected {
					t.Errorf("Expected prompt %q, got %q", expected, *cond.Then.Prompt)
				}
			},
		},
		{
			name: "conditional with regular prompt (no prefix)",
			jsonData: `{
				"story": "Test scene",
				"conditionals": {
					"room_cold": {
						"when": {
							"vars": {"window_open": "true"}
						},
						"then": {
							"prompt": "The room grows noticeably colder."
						}
					}
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				cond := scene.Conditionals["room_cold"]
				if cond.Then.Prompt == nil {
					t.Error("Expected prompt to be set")
					return
				}
				if *cond.Then.Prompt != "The room grows noticeably colder." {
					t.Errorf("Unexpected prompt: %q", *cond.Then.Prompt)
				}
			},
		},
		{
			name: "conditional with scene change (no prompt)",
			jsonData: `{
				"story": "Test scene",
				"conditionals": {
					"advance_scene": {
						"when": {
							"vars": {"quest_complete": "true"}
						},
						"then": {
							"scene_change": {"to": "next_scene", "reason": "conditional"}
						}
					}
				}
			}`,
			expectError: false,
			validate: func(t *testing.T, scene Scene) {
				cond := scene.Conditionals["advance_scene"]
				if cond.Then.Prompt != nil {
					t.Error("Expected prompt to be nil")
				}
				if cond.Then.SceneChange == nil {
					t.Fatal("Expected SceneChange to be non-nil")
				}
				if cond.Then.SceneChange.To != "next_scene" {
					t.Errorf("Expected scene 'next_scene', got %q", cond.Then.SceneChange.To)
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
