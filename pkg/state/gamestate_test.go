package state

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
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

The following JSON describes the complete world and current state.

Game State:
` + "```json\n" + `{"locations":{"Tortuga":{"name":"Tortuga","description":"A pirate port","exits":{"east":"Black Pearl"}}},"user_location":"Tortuga","user_inventory":["cutlass","spyglass"],"is_ended":false}
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
			if tt.gameState.SceneName != "" {
				err := tt.gameState.LoadScene(tt.scenario, tt.gameState.SceneName)
				if err != nil {
					if !tt.expectError {
						t.Fatalf("Unexpected error loading scene: %v", err)
					}
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
				NPCs: map[string]actor.NPC{
					"TestNPC": {
						Name:     "TestNPC",
						Location: "TestLocation",
					},
				},
				ContingencyPrompts: []conditionals.ContingencyPrompt{{Prompt: "Scene contingency"}},
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
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "Scene prompt 1"},
							{Prompt: "Scene prompt 2"},
						},
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
		{
			name: "PC-level prompts included",
			gameState: &GameState{
				ContingencyPrompts: []string{"Scenario prompt"},
				PC: &actor.PC{
					Spec: &actor.PCSpec{
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "PC prompt 1"},
							{Prompt: "PC prompt 2"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{},
			expected: []string{"PC prompt 1", "PC prompt 2", "Scenario prompt"},
		},
		{
			name: "PC with conditional prompts",
			gameState: &GameState{
				Vars:               map[string]string{"has_sword": "true"},
				TurnCounter:        15,
				ContingencyPrompts: []string{},
				PC: &actor.PC{
					Spec: &actor.PCSpec{
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "PC is always brave"},
							{
								Prompt: "PC is confident with sword",
								When:   &conditionals.ConditionalWhen{Vars: map[string]string{"has_sword": "true"}},
							},
							{
								Prompt: "PC is tired after many turns",
								When: &conditionals.ConditionalWhen{
									MinTurns: func() *int { i := 20; return &i }(),
								},
							},
						},
					},
				},
			},
			scenario: &scenario.Scenario{},
			expected: []string{"PC is always brave", "PC is confident with sword"},
		},
		{
			name: "All levels combined: scenario, PC, gamestate, scene",
			gameState: &GameState{
				SceneName:          "test_scene",
				ContingencyPrompts: []string{"Gamestate custom prompt"},
				PC: &actor.PC{
					Spec: &actor.PCSpec{
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "PC prompt"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				ContingencyPrompts: []conditionals.ContingencyPrompt{
					{Prompt: "Scenario prompt"},
				},
				Scenes: map[string]scenario.Scene{
					"test_scene": {
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "Scene prompt"},
						},
					},
				},
			},
			expected: []string{"Scenario prompt", "PC prompt", "Gamestate custom prompt", "Scene prompt"},
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
				NPCs: map[string]actor.NPC{
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
				NPCs: map[string]actor.NPC{
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
				NPCs: map[string]actor.NPC{
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
				NPCs: map[string]actor.NPC{
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
				NPCs: map[string]actor.NPC{
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
				NPCs:           map[string]actor.NPC{},
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

func TestGameState_GetStoryEvents(t *testing.T) {
	tests := []struct {
		name     string
		gs       *GameState
		expected string
	}{
		{
			name: "empty queue",
			gs: &GameState{
				StoryEventQueue: []string{},
			},
			expected: "",
		},
		{
			name: "single event",
			gs: &GameState{
				StoryEventQueue: []string{"Count Dracula materializes from the shadows, his eyes burning with ancient hunger."},
			},
			expected: "STORY EVENT: Count Dracula materializes from the shadows, his eyes burning with ancient hunger.",
		},
		{
			name: "multiple events",
			gs: &GameState{
				StoryEventQueue: []string{
					"Count Dracula materializes from the shadows, his eyes burning with ancient hunger.",
					"A massive LIGHTNING bolt strikes the castle tower! Thunder shakes the stones!",
				},
			},
			expected: "STORY EVENT: Count Dracula materializes from the shadows, his eyes burning with ancient hunger.\n\nSTORY EVENT: A massive LIGHTNING bolt strikes the castle tower! Thunder shakes the stones!",
		},
		{
			name: "three events",
			gs: &GameState{
				StoryEventQueue: []string{
					"Event one.",
					"Event two.",
					"Event three.",
				},
			},
			expected: "STORY EVENT: Event one.\n\nSTORY EVENT: Event two.\n\nSTORY EVENT: Event three.",
		},
		{
			name: "nil queue",
			gs: &GameState{
				StoryEventQueue: nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.gs.GetStoryEvents()
			if result != tt.expected {
				t.Errorf("GetStoryEvents() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestGameState_ClearStoryEventQueue(t *testing.T) {
	tests := []struct {
		name         string
		initialQueue []string
	}{
		{
			name:         "clear empty queue",
			initialQueue: []string{},
		},
		{
			name:         "clear single event",
			initialQueue: []string{"Event one"},
		},
		{
			name:         "clear multiple events",
			initialQueue: []string{"Event one", "Event two", "Event three"},
		},
		{
			name:         "clear nil queue",
			initialQueue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := &GameState{
				StoryEventQueue: tt.initialQueue,
			}

			gs.ClearStoryEventQueue()

			if len(gs.StoryEventQueue) != 0 {
				t.Errorf("ClearStoryEventQueue() left %d events in queue, expected 0", len(gs.StoryEventQueue))
			}

			// Verify it's an empty slice, not nil (for consistency)
			if gs.StoryEventQueue == nil {
				t.Error("ClearStoryEventQueue() resulted in nil queue, expected empty slice")
			}
		})
	}
}

func TestGameState_StoryEventQueue_Persistence(t *testing.T) {
	// Test that story event queue persists through serialization/deserialization
	gs := NewGameState("test.json", nil, "test-model")
	gs.StoryEventQueue = []string{
		"Event one",
		"Event two",
	}

	// Serialize
	data, err := json.Marshal(gs)
	if err != nil {
		t.Fatalf("Failed to marshal GameState: %v", err)
	}

	// Deserialize
	var restored GameState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal GameState: %v", err)
	}

	// Verify queue persisted
	if len(restored.StoryEventQueue) != len(gs.StoryEventQueue) {
		t.Errorf("Expected %d events in queue after deserialization, got %d", len(gs.StoryEventQueue), len(restored.StoryEventQueue))
	}

	for i, event := range gs.StoryEventQueue {
		if restored.StoryEventQueue[i] != event {
			t.Errorf("Event %d: expected %q, got %q", i, event, restored.StoryEventQueue[i])
		}
	}
}

func TestGameState_StoryEventQueue_EnqueueDequeue(t *testing.T) {
	gs := NewGameState("test.json", nil, "test-model")

	// Initially empty
	if len(gs.StoryEventQueue) != 0 {
		t.Errorf("Expected empty queue initially, got %d events", len(gs.StoryEventQueue))
	}

	// Enqueue first event
	event1 := "First event"
	gs.StoryEventQueue = append(gs.StoryEventQueue, event1)
	if len(gs.StoryEventQueue) != 1 {
		t.Errorf("Expected 1 event after first enqueue, got %d", len(gs.StoryEventQueue))
	}

	// Enqueue second event
	event2 := "Second event"
	gs.StoryEventQueue = append(gs.StoryEventQueue, event2)
	if len(gs.StoryEventQueue) != 2 {
		t.Errorf("Expected 2 events after second enqueue, got %d", len(gs.StoryEventQueue))
	}

	// Verify order (FIFO)
	if gs.StoryEventQueue[0] != event1 {
		t.Errorf("Expected first event to be %q, got %q", event1, gs.StoryEventQueue[0])
	}
	if gs.StoryEventQueue[1] != event2 {
		t.Errorf("Expected second event to be %q, got %q", event2, gs.StoryEventQueue[1])
	}

	// Dequeue (via GetStoryEvents and Clear)
	formattedEvents := gs.GetStoryEvents()
	expectedFormat := "STORY EVENT: First event\n\nSTORY EVENT: Second event"
	if formattedEvents != expectedFormat {
		t.Errorf("Expected formatted events %q, got %q", expectedFormat, formattedEvents)
	}

	gs.ClearStoryEventQueue()
	if len(gs.StoryEventQueue) != 0 {
		t.Errorf("Expected empty queue after clear, got %d events", len(gs.StoryEventQueue))
	}
}

func TestGameState_GetChatMessages_WithStoryEvents(t *testing.T) {
	tests := []struct {
		name             string
		storyEventPrompt string
		userMessage      string
		expectedContains []string
		expectedOrder    []string
		description      string
	}{
		{
			name:             "no story events",
			storyEventPrompt: "",
			userMessage:      "I look around",
			expectedContains: []string{"I look around"},
			description:      "should not inject story event when prompt is empty",
		},
		{
			name:             "single story event",
			storyEventPrompt: "STORY EVENT: Count Dracula appears from the shadows.",
			userMessage:      "I look around",
			expectedContains: []string{
				"I look around",
				"STORY EVENT: Count Dracula appears from the shadows.",
			},
			expectedOrder: []string{
				"I look around",
				"STORY EVENT: Count Dracula appears from the shadows.",
			},
			description: "should inject story event after user message",
		},
		{
			name:             "multiple story events",
			storyEventPrompt: "STORY EVENT: Count Dracula appears.\n\nSTORY EVENT: Lightning strikes!",
			userMessage:      "I open the door",
			expectedContains: []string{
				"I open the door",
				"STORY EVENT: Count Dracula appears.\n\nSTORY EVENT: Lightning strikes!",
			},
			expectedOrder: []string{
				"I open the door",
				"STORY EVENT: Count Dracula appears.\n\nSTORY EVENT: Lightning strikes!",
			},
			description: "should inject multiple story events after user message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := NewGameState("test.json", nil, "test-model")
			gs.SceneName = "test_scene"
			gs.Location = "test_location"

			scenario := &scenario.Scenario{
				Name:   "Test Scenario",
				Story:  "A test scenario",
				Rating: scenario.RatingPG,
				Scenes: map[string]scenario.Scene{
					"test_scene": {
						Story: "Test scene story",
						Locations: map[string]scenario.Location{
							"test_location": {
								Name:        "test_location",
								Description: "A test location",
							},
						},
					},
				},
			}

			err := gs.LoadScene(scenario, "test_scene")
			if err != nil {
				t.Fatalf("Failed to load scene: %v", err)
			}

			messages, err := gs.GetChatMessages(tt.userMessage, chat.ChatRoleUser, scenario, 10, tt.storyEventPrompt)
			if err != nil {
				t.Fatalf("GetChatMessages failed: %v", err)
			}

			// Verify all expected strings are present
			for _, expected := range tt.expectedContains {
				found := false
				for _, msg := range messages {
					if strings.Contains(msg.Content, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find %q in messages, but it was not present", expected)
				}
			}

			// Verify order if specified
			if len(tt.expectedOrder) > 0 {
				// Find indices of expected strings
				indices := make(map[string]int)
				for i, msg := range messages {
					for _, expected := range tt.expectedOrder {
						if strings.Contains(msg.Content, expected) {
							indices[expected] = i
						}
					}
				}

				// Verify order
				for i := 1; i < len(tt.expectedOrder); i++ {
					prev := tt.expectedOrder[i-1]
					curr := tt.expectedOrder[i]

					prevIdx, prevFound := indices[prev]
					currIdx, currFound := indices[curr]

					if !prevFound {
						t.Errorf("Expected string %q not found in messages", prev)
					}
					if !currFound {
						t.Errorf("Expected string %q not found in messages", curr)
					}

					if prevFound && currFound && prevIdx >= currIdx {
						t.Errorf("Expected %q (index %d) to come before %q (index %d)", prev, prevIdx, curr, currIdx)
					}
				}
			}
		})
	}
}

func TestGameState_GetChatMessages_StoryEventPosition(t *testing.T) {
	// This test specifically validates that story events are injected at the correct position:
	// After the user's message but before the final system reminders
	gs := NewGameState("test.json", nil, "test-model")
	gs.SceneName = "test_scene"
	gs.Location = "test_location"

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test scenario",
		Rating: scenario.RatingPG,
		Scenes: map[string]scenario.Scene{
			"test_scene": {
				Story: "Test scene story",
				Locations: map[string]scenario.Location{
					"test_location": {
						Name:        "test_location",
						Description: "A test location",
					},
				},
			},
		},
	}

	err := gs.LoadScene(scenario, "test_scene")
	if err != nil {
		t.Fatalf("Failed to load scene: %v", err)
	}

	storyEventPrompt := "STORY EVENT: A dragon appears!"
	userMessage := "I draw my sword"

	messages, err := gs.GetChatMessages(userMessage, chat.ChatRoleUser, scenario, 10, storyEventPrompt)
	if err != nil {
		t.Fatalf("GetChatMessages failed: %v", err)
	}

	// Find indices
	var userMsgIdx, storyEventIdx, finalReminderIdx = -1, -1, -1

	for i, msg := range messages {
		if msg.Role == chat.ChatRoleUser && strings.Contains(msg.Content, userMessage) {
			userMsgIdx = i
		}
		if msg.Role == chat.ChatRoleAgent && strings.Contains(msg.Content, storyEventPrompt) {
			storyEventIdx = i
		}
		// Final reminder is the last system message
		if msg.Role == chat.ChatRoleSystem && i == len(messages)-1 {
			finalReminderIdx = i
		}
	}

	// Verify indices are valid
	if userMsgIdx == -1 {
		t.Error("User message not found in messages")
	}
	if storyEventIdx == -1 {
		t.Error("Story event not found in messages")
	}
	if finalReminderIdx == -1 {
		t.Error("Final reminder not found in messages")
	}

	// Verify order: user message < story event < final reminder
	if userMsgIdx >= storyEventIdx {
		t.Errorf("Story event (index %d) should come after user message (index %d)", storyEventIdx, userMsgIdx)
	}
	if storyEventIdx >= finalReminderIdx {
		t.Errorf("Final reminder (index %d) should come after story event (index %d)", finalReminderIdx, storyEventIdx)
	}
}

func TestGameState_GetChatMessages_NoStoryEventWhenEmpty(t *testing.T) {
	gs := NewGameState("test.json", nil, "test-model")
	gs.SceneName = "test_scene"
	gs.Location = "test_location"

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test scenario",
		Rating: scenario.RatingPG,
		Scenes: map[string]scenario.Scene{
			"test_scene": {
				Story: "Test scene story",
				Locations: map[string]scenario.Location{
					"test_location": {
						Name:        "test_location",
						Description: "A test location",
					},
				},
			},
		},
	}

	err := gs.LoadScene(scenario, "test_scene")
	if err != nil {
		t.Fatalf("Failed to load scene: %v", err)
	}

	messages, err := gs.GetChatMessages("I look around", chat.ChatRoleUser, scenario, 10, "")
	if err != nil {
		t.Fatalf("GetChatMessages failed: %v", err)
	}

	// Verify no story event message is present (agent/assistant role message)
	for _, msg := range messages {
		if msg.Role == chat.ChatRoleAgent && strings.Contains(msg.Content, "STORY EVENT:") {
			t.Error("Found STORY EVENT agent message when storyEventPrompt was empty")
		}
	}
}

func TestGameState_GetContingencyPrompts_WithNPCs(t *testing.T) {
	tests := []struct {
		name              string
		gameState         *GameState
		scenario          *scenario.Scenario
		expectedPrompts   []string
		unexpectedPrompts []string
	}{
		{
			name: "NPC at same location shows prompts",
			gameState: &GameState{
				Location: "tavern",
				NPCs: map[string]actor.NPC{
					"bartender": {
						Name:     "Bartender",
						Location: "tavern",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The bartender wipes down glasses"},
							{Prompt: "The tavern smells of ale"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			expectedPrompts: []string{
				"The bartender wipes down glasses",
				"The tavern smells of ale",
			},
		},
		{
			name: "NPC at different location does not show prompts",
			gameState: &GameState{
				Location: "market",
				NPCs: map[string]actor.NPC{
					"bartender": {
						Name:     "Bartender",
						Location: "tavern",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The bartender wipes down glasses"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			unexpectedPrompts: []string{
				"The bartender wipes down glasses",
			},
		},
		{
			name: "NPC conditional prompts based on vars",
			gameState: &GameState{
				Location: "tavern",
				Vars: map[string]string{
					"met_bartender": "true",
					"bar_tab_paid":  "false",
				},
				NPCs: map[string]actor.NPC{
					"bartender": {
						Name:     "Bartender",
						Location: "tavern",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The bartender is always friendly"},
							{
								Prompt: "The bartender greets you warmly",
								When: &conditionals.ConditionalWhen{
									Vars: map[string]string{"met_bartender": "true"},
								},
							},
							{
								Prompt: "The bartender looks at you suspiciously",
								When: &conditionals.ConditionalWhen{
									Vars: map[string]string{"met_bartender": "false"},
								},
							},
							{
								Prompt: "The bartender taps the bar, waiting for payment",
								When: &conditionals.ConditionalWhen{
									Vars: map[string]string{
										"met_bartender": "true",
										"bar_tab_paid":  "false",
									},
								},
							},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			expectedPrompts: []string{
				"The bartender is always friendly",
				"The bartender greets you warmly",
				"The bartender taps the bar, waiting for payment",
			},
			unexpectedPrompts: []string{
				"The bartender looks at you suspiciously",
			},
		},
		{
			name: "Multiple NPCs at same location",
			gameState: &GameState{
				Location: "market",
				NPCs: map[string]actor.NPC{
					"merchant": {
						Name:     "Merchant",
						Location: "market",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The merchant displays exotic wares"},
						},
					},
					"guard": {
						Name:     "Guard",
						Location: "market",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The guard watches carefully"},
						},
					},
					"innkeeper": {
						Name:     "Innkeeper",
						Location: "inn",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "The innkeeper tends the fire"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
			},
			expectedPrompts: []string{
				"The merchant displays exotic wares",
				"The guard watches carefully",
			},
			unexpectedPrompts: []string{
				"The innkeeper tends the fire",
			},
		},
		{
			name: "NPC prompts combined with scenario and scene prompts",
			gameState: &GameState{
				Location:  "tavern",
				SceneName: "opening",
				NPCs: map[string]actor.NPC{
					"bartender": {
						Name:     "Bartender",
						Location: "tavern",
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "NPC prompt from bartender"},
						},
					},
				},
			},
			scenario: &scenario.Scenario{
				Name: "Test",
				ContingencyPrompts: []conditionals.ContingencyPrompt{
					{Prompt: "Scenario-level prompt"},
				},
				Scenes: map[string]scenario.Scene{
					"opening": {
						ContingencyPrompts: []conditionals.ContingencyPrompt{
							{Prompt: "Scene-level prompt"},
						},
					},
				},
			},
			expectedPrompts: []string{
				"Scenario-level prompt",
				"Scene-level prompt",
				"NPC prompt from bartender",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompts := tt.gameState.GetContingencyPrompts(tt.scenario)

			// Check expected prompts are present
			for _, expected := range tt.expectedPrompts {
				found := false
				for _, prompt := range prompts {
					if prompt == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected prompt not found: %q", expected)
				}
			}

			// Check unexpected prompts are absent
			for _, unexpected := range tt.unexpectedPrompts {
				for _, prompt := range prompts {
					if prompt == unexpected {
						t.Errorf("Unexpected prompt found: %q", unexpected)
					}
				}
			}
		})
	}
}
