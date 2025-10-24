package storage

import (
	"context"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/scenario"
)

func TestMockStorage_AddAndGetScenario(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// Add a test scenario
	testScenario := &scenario.Scenario{
		Name:            "Test Adventure",
		FileName:        "test_adventure.json",
		Story:           "A thrilling test adventure",
		OpeningPrompt:   "Welcome adventurer!",
		OpeningLocation: "town_square",
		Locations: map[string]scenario.Location{
			"town_square": {
				Name:        "town_square",
				Description: "The bustling town square",
				Exits: map[string]string{
					"north": "tavern",
				},
			},
		},
	}

	mockStorage.AddScenario("test_adventure.json", testScenario)

	// Get it back
	loaded, err := mockStorage.GetScenario(ctx, "test_adventure.json")
	if err != nil {
		t.Fatalf("Failed to get scenario: %v", err)
	}

	if loaded.Name != "Test Adventure" {
		t.Errorf("Expected name 'Test Adventure', got %v", loaded.Name)
	}

	if loaded.FileName != "test_adventure.json" {
		t.Errorf("Expected filename 'test_adventure.json', got %v", loaded.FileName)
	}

	if loaded.OpeningLocation != "town_square" {
		t.Errorf("Expected opening location 'town_square', got %v", loaded.OpeningLocation)
	}
}

func TestMockStorage_GetNonExistentScenario(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// Try to get a non-existent scenario
	_, err := mockStorage.GetScenario(ctx, "nonexistent.json")
	if err == nil {
		t.Error("Expected error for non-existent scenario")
	}

	if err.Error() != "scenario not found" {
		t.Errorf("Expected 'scenario not found' error, got: %v", err)
	}
}

func TestMockStorage_ListScenarios(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// Add multiple scenarios
	mockStorage.AddScenario("adventure1.json", &scenario.Scenario{
		Name:     "Adventure 1",
		FileName: "adventure1.json",
	})
	mockStorage.AddScenario("adventure2.json", &scenario.Scenario{
		Name:     "Adventure 2",
		FileName: "adventure2.json",
	})
	mockStorage.AddScenario("quest.json", &scenario.Scenario{
		Name:     "Epic Quest",
		FileName: "quest.json",
	})

	// List them
	scenarios, err := mockStorage.ListScenarios(ctx)
	if err != nil {
		t.Fatalf("Failed to list scenarios: %v", err)
	}

	if len(scenarios) != 3 {
		t.Errorf("Expected 3 scenarios, got %d", len(scenarios))
	}

	// Check that all names are mapped to filenames
	if scenarios["Adventure 1"] != "adventure1.json" {
		t.Errorf("Expected 'Adventure 1' -> 'adventure1.json', got %v", scenarios["Adventure 1"])
	}

	if scenarios["Adventure 2"] != "adventure2.json" {
		t.Errorf("Expected 'Adventure 2' -> 'adventure2.json', got %v", scenarios["Adventure 2"])
	}

	if scenarios["Epic Quest"] != "quest.json" {
		t.Errorf("Expected 'Epic Quest' -> 'quest.json', got %v", scenarios["Epic Quest"])
	}
}

func TestMockStorage_ListScenariosEmpty(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// List scenarios when none exist
	scenarios, err := mockStorage.ListScenarios(ctx)
	if err != nil {
		t.Fatalf("Failed to list scenarios: %v", err)
	}

	if len(scenarios) != 0 {
		t.Errorf("Expected 0 scenarios, got %d", len(scenarios))
	}
}
