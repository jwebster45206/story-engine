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
		expected    chat.ChatMessage
		expectError bool
	}{
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
			expected: chat.ChatMessage{
				Role: chat.ChatRoleSystem,
				Content: `The user is roleplaying this scenario: A test adventure

The following JSON describes the complete world and current state.

Game State:
` + "```json\n" + `{"npcs":{"Gibbs":{"name":"Gibbs","type":"pirate","disposition":"loyal","location":"Black Pearl"}},"locations":{"Tortuga":{"name":"Tortuga","description":"A pirate port","exits":{"east":"Black Pearl"}}},"user_location":"Tortuga","user_inventory":["cutlass","spyglass"],"is_ended":false}
` + "```",
			},
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
			expected: chat.ChatMessage{
				Role: chat.ChatRoleSystem,
				Content: `The user is roleplaying this scenario: Overall pirate story

Find the shipwright

The following JSON describes the complete world and current state.

Game State:
` + "```json\n" + `{"npcs":{"Shipwright":{"name":"Shipwright","type":"craftsman","disposition":"gruff","location":"Tortuga"}},"locations":{"Tortuga":{"name":"Tortuga","description":"A bustling pirate port","exits":{"east":"Black Pearl"}}},"user_location":"Tortuga","user_inventory":["cutlass"],"is_ended":false}
` + "```",
			},
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.gameState.LoadScene(tt.scenario, tt.gameState.SceneName)
			if err != nil {
				if !tt.expectError {
					t.Fatalf("Unexpected error loading scene: %v", err)
				}
				return
			}
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

	err := gameState.LoadScene(scenario, "test_scene")
	if err != nil {
		t.Fatalf("Error loading scene: %v", err)
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
}

func TestGameState_GetContingencyPrompts(t *testing.T) {
	tests := []struct {
		name      string
		gameState *GameState
		scenario  *scenario.Scenario
		expected  []string
	}{
		{
			name:      "nil gamestate",
			gameState: nil,
			scenario:  &scenario.Scenario{},
			expected:  nil,
		},
		{
			name:      "nil scenario",
			gameState: &GameState{},
			scenario:  nil,
			expected:  nil,
		},
		{
			name: "scenario-level prompts only",
			gameState: &GameState{
				ContingencyPrompts: []string{"Scenario prompt 1", "Scenario prompt 2"},
			},
			scenario: &scenario.Scenario{},
			expected: []string{"Scenario prompt 1", "Scenario prompt 2"},
		},
		{
			name: "scene-level prompts added",
			gameState: &GameState{
				SceneName:          "test_scene",
				ContingencyPrompts: []string{"Scenario prompt"},
			},
			scenario: &scenario.Scenario{
				Scenes: map[string]scenario.Scene{
					"test_scene": {
						ContingencyPrompts: []string{"Scene prompt 1", "Scene prompt 2"},
					},
				},
			},
			expected: []string{"Scenario prompt", "Scene prompt 1", "Scene prompt 2"},
		},
		{
			name: "scene not found",
			gameState: &GameState{
				SceneName:          "nonexistent_scene",
				ContingencyPrompts: []string{"Scenario prompt"},
			},
			scenario: &scenario.Scenario{
				Scenes: map[string]scenario.Scene{},
			},
			expected: []string{"Scenario prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.gameState.GetContingencyPrompts(tt.scenario)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d prompts, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected prompt %d to be '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestGameState_NormalizeItems(t *testing.T) {
	tests := []struct {
		name              string
		gameState         *GameState
		expectedInventory []string
		expectedNPCItems  map[string][]string
		expectedLocItems  map[string][]string
		description       string
	}{
		{
			name:        "nil gamestate",
			gameState:   nil,
			description: "should handle nil gamestate gracefully",
		},
		{
			name: "no duplicates",
			gameState: &GameState{
				Inventory: []string{"sword", "shield"},
				NPCs: map[string]scenario.NPC{
					"guard":    {Items: []string{"key", "armor"}},
					"merchant": {Items: []string{"potion", "gold"}},
				},
				WorldLocations: map[string]scenario.Location{
					"cave":   {Items: []string{"gem", "torch"}},
					"forest": {Items: []string{"berries", "wood"}},
				},
			},
			expectedInventory: []string{"sword", "shield"},
			expectedNPCItems: map[string][]string{
				"guard":    {"key", "armor"},
				"merchant": {"potion", "gold"},
			},
			expectedLocItems: map[string][]string{
				"cave":   {"gem", "torch"},
				"forest": {"berries", "wood"},
			},
			description: "should not remove any items when no duplicates exist",
		},
		{
			name: "user inventory takes priority over NPCs",
			gameState: &GameState{
				Inventory: []string{"sword", "key"},
				NPCs: map[string]scenario.NPC{
					"guard":    {Items: []string{"key", "armor", "sword"}},
					"merchant": {Items: []string{"potion", "key"}},
				},
				WorldLocations: map[string]scenario.Location{
					"cave": {Items: []string{"gem", "torch"}},
				},
			},
			expectedInventory: []string{"sword", "key"},
			expectedNPCItems: map[string][]string{
				"guard":    {"armor"},
				"merchant": {"potion"},
			},
			expectedLocItems: map[string][]string{
				"cave": {"gem", "torch"},
			},
			description: "should remove items from NPCs when they exist in user inventory",
		},
		{
			name: "user inventory takes priority over locations",
			gameState: &GameState{
				Inventory: []string{"sword", "gem"},
				NPCs: map[string]scenario.NPC{
					"guard": {Items: []string{"key", "armor"}},
				},
				WorldLocations: map[string]scenario.Location{
					"cave":   {Items: []string{"gem", "torch", "sword"}},
					"forest": {Items: []string{"berries", "gem"}},
				},
			},
			expectedInventory: []string{"sword", "gem"},
			expectedNPCItems: map[string][]string{
				"guard": {"key", "armor"},
			},
			expectedLocItems: map[string][]string{
				"cave":   {"torch"},
				"forest": {"berries"},
			},
			description: "should remove items from locations when they exist in user inventory",
		},
		{
			name: "NPC items take priority over locations",
			gameState: &GameState{
				Inventory: []string{"sword"},
				NPCs: map[string]scenario.NPC{
					"guard":    {Items: []string{"key", "armor"}},
					"merchant": {Items: []string{"potion", "gem"}},
				},
				WorldLocations: map[string]scenario.Location{
					"cave":   {Items: []string{"gem", "torch", "key"}},
					"forest": {Items: []string{"berries", "armor"}},
				},
			},
			expectedInventory: []string{"sword"},
			expectedNPCItems: map[string][]string{
				"guard":    {"key", "armor"},
				"merchant": {"potion", "gem"},
			},
			expectedLocItems: map[string][]string{
				"cave":   {"torch"},
				"forest": {"berries"},
			},
			description: "should remove items from locations when they exist with NPCs",
		},
		{
			name: "complex scenario with all priorities",
			gameState: &GameState{
				Inventory: []string{"legendary_sword", "master_key"},
				NPCs: map[string]scenario.NPC{
					"guard":    {Items: []string{"iron_key", "chain_mail", "legendary_sword"}},
					"merchant": {Items: []string{"health_potion", "master_key", "gold_coin"}},
					"wizard":   {Items: []string{"spell_book", "iron_key"}},
				},
				WorldLocations: map[string]scenario.Location{
					"castle":  {Items: []string{"legendary_sword", "crown", "iron_key"}},
					"dungeon": {Items: []string{"master_key", "torch", "health_potion"}},
					"shop":    {Items: []string{"bread", "gold_coin"}},
				},
			},
			// For complex test, we'll validate the singleton behavior rather than exact NPC assignments
			expectedInventory: []string{"legendary_sword", "master_key"},
			description:       "should handle complex scenarios with multiple overlaps correctly",
		},
		{
			name: "empty collections",
			gameState: &GameState{
				Inventory:      []string{},
				NPCs:           map[string]scenario.NPC{},
				WorldLocations: map[string]scenario.Location{},
			},
			expectedInventory: []string{},
			expectedNPCItems:  map[string][]string{},
			expectedLocItems:  map[string][]string{},
			description:       "should handle empty collections without issues",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the test data
			var testGameState *GameState
			if tt.gameState != nil {
				copied, err := tt.gameState.DeepCopy()
				if err != nil {
					t.Fatalf("Failed to copy gamestate: %v", err)
				}
				testGameState = copied
			}

			// Execute the function
			testGameState.NormalizeItems()

			// Skip validation for nil gamestate test
			if tt.gameState == nil {
				return
			}

			// Check user inventory
			if !stringSlicesEqual(testGameState.Inventory, tt.expectedInventory) {
				t.Errorf("Expected user inventory %v, got %v", tt.expectedInventory, testGameState.Inventory)
			}

			// For the complex scenario, validate singleton behavior instead of exact assignments
			if tt.name == "complex scenario with all priorities" {
				// Validate that singleton behavior is enforced
				validateItemSingletons(t, testGameState, tt.gameState)
			} else {
				// Check NPC items for simpler test cases
				for npcName, expectedItems := range tt.expectedNPCItems {
					if npc, exists := testGameState.NPCs[npcName]; !exists {
						t.Errorf("Expected NPC '%s' to exist", npcName)
					} else if !stringSlicesEqual(npc.Items, expectedItems) {
						t.Errorf("Expected NPC '%s' items %v, got %v", npcName, expectedItems, npc.Items)
					}
				}

				// Check location items for simpler test cases
				for locName, expectedItems := range tt.expectedLocItems {
					if loc, exists := testGameState.WorldLocations[locName]; !exists {
						t.Errorf("Expected location '%s' to exist", locName)
					} else if !stringSlicesEqual(loc.Items, expectedItems) {
						t.Errorf("Expected location '%s' items %v, got %v", locName, expectedItems, loc.Items)
					}
				}
			}
		})
	}
}

// Helper function to compare string slices
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// validateItemSingletons verifies that the item singleton behavior is enforced
// It checks that no item appears in multiple places and that items are prioritized correctly
func validateItemSingletons(t *testing.T, normalizedState *GameState, originalState *GameState) {
	// Collect all items from normalized state
	allItems := make(map[string]string) // item -> location type (user/npc/location)

	// Track user items (highest priority)
	for _, item := range normalizedState.Inventory {
		allItems[item] = "user"
	}

	// Track NPC items (second priority)
	for npcName, npc := range normalizedState.NPCs {
		for _, item := range npc.Items {
			if existing, exists := allItems[item]; exists {
				t.Errorf("Item '%s' appears in both %s and NPC '%s'", item, existing, npcName)
			}
			allItems[item] = "npc:" + npcName
		}
	}

	// Track location items (lowest priority)
	for locName, location := range normalizedState.WorldLocations {
		for _, item := range location.Items {
			if existing, exists := allItems[item]; exists {
				t.Errorf("Item '%s' appears in both %s and location '%s'", item, existing, locName)
			}
			allItems[item] = "location:" + locName
		}
	}

	// Verify priority enforcement: items in user inventory should not appear anywhere else
	for _, userItem := range normalizedState.Inventory {
		for _, originalUserItem := range originalState.Inventory {
			if userItem == originalUserItem {
				// This item was originally in user inventory, so it should stay there
				// Check that it's been removed from NPCs and locations
				for npcName, npc := range normalizedState.NPCs {
					for _, npcItem := range npc.Items {
						if npcItem == userItem {
							t.Errorf("Item '%s' should be removed from NPC '%s' because it's in user inventory", userItem, npcName)
						}
					}
				}
				for locName, location := range normalizedState.WorldLocations {
					for _, locItem := range location.Items {
						if locItem == userItem {
							t.Errorf("Item '%s' should be removed from location '%s' because it's in user inventory", userItem, locName)
						}
					}
				}
				break
			}
		}
	}

	// Verify that all expected items still exist somewhere (no items should be lost)
	originalItems := make(map[string]bool)
	for _, item := range originalState.Inventory {
		originalItems[item] = true
	}
	for _, npc := range originalState.NPCs {
		for _, item := range npc.Items {
			originalItems[item] = true
		}
	}
	for _, location := range originalState.WorldLocations {
		for _, item := range location.Items {
			originalItems[item] = true
		}
	}

	for originalItem := range originalItems {
		if _, exists := allItems[originalItem]; !exists {
			t.Errorf("Item '%s' was lost during normalization", originalItem)
		}
	}
}
