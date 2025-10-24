package storage

import (
	"context"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
)

func TestMockStorage_AddAndGetPCSpec(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// Add a test PC spec
	testPC := &actor.PCSpec{
		ID:    "warrior",
		Name:  "Brave Warrior",
		Class: "fighter",
		Level: 5,
		Race:  "human",
		Stats: actor.Stats5e{
			Strength:     18,
			Dexterity:    14,
			Constitution: 16,
			Intelligence: 10,
			Wisdom:       12,
			Charisma:     8,
		},
		HP:    45,
		MaxHP: 45,
	}

	mockStorage.AddPCSpec("warrior", testPC)

	// Get it back by ID
	loaded, err := mockStorage.GetPCSpec(ctx, "warrior")
	if err != nil {
		t.Fatalf("Failed to get PC spec by ID: %v", err)
	}

	if loaded.ID != "warrior" {
		t.Errorf("Expected ID 'warrior', got %v", loaded.ID)
	}

	if loaded.Name != "Brave Warrior" {
		t.Errorf("Expected name 'Brave Warrior', got %v", loaded.Name)
	}

	if loaded.Class != "fighter" {
		t.Errorf("Expected class 'fighter', got %v", loaded.Class)
	}

	if loaded.Stats.Strength != 18 {
		t.Errorf("Expected strength 18, got %d", loaded.Stats.Strength)
	}

	// Get it back by path (MockStorage should handle path matching)
	loaded2, err := mockStorage.GetPCSpec(ctx, "../../data/pcs/warrior.json")
	if err != nil {
		t.Fatalf("Failed to get PC spec by path: %v", err)
	}

	if loaded2.ID != "warrior" {
		t.Errorf("Expected ID 'warrior' when using path, got %v", loaded2.ID)
	}
}

func TestMockStorage_GetNonExistentPCSpec(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// Try to get a non-existent PC
	_, err := mockStorage.GetPCSpec(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent PC")
	}

	if err.Error() != "PC spec not found" {
		t.Errorf("Expected 'PC spec not found' error, got: %v", err)
	}
}

func TestMockStorage_ListPCs(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// Add multiple PCs
	mockStorage.AddPCSpec("warrior", &actor.PCSpec{
		ID:    "warrior",
		Name:  "Brave Warrior",
		Class: "fighter",
	})
	mockStorage.AddPCSpec("mage", &actor.PCSpec{
		ID:    "mage",
		Name:  "Wise Mage",
		Class: "wizard",
	})
	mockStorage.AddPCSpec("rogue", &actor.PCSpec{
		ID:    "rogue",
		Name:  "Sneaky Rogue",
		Class: "rogue",
	})

	// List them
	pcs, err := mockStorage.ListPCs(ctx)
	if err != nil {
		t.Fatalf("Failed to list PCs: %v", err)
	}

	if len(pcs) != 3 {
		t.Errorf("Expected 3 PCs, got %d", len(pcs))
	}

	// Check that all IDs are present
	found := make(map[string]bool)
	for _, id := range pcs {
		found[id] = true
	}

	if !found["warrior"] {
		t.Error("Expected to find 'warrior' PC")
	}
	if !found["mage"] {
		t.Error("Expected to find 'mage' PC")
	}
	if !found["rogue"] {
		t.Error("Expected to find 'rogue' PC")
	}
}

func TestMockStorage_ListPCsEmpty(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// List PCs when none exist
	pcs, err := mockStorage.ListPCs(ctx)
	if err != nil {
		t.Fatalf("Failed to list PCs: %v", err)
	}

	if len(pcs) != 0 {
		t.Errorf("Expected 0 PCs, got %d", len(pcs))
	}
}

func TestMockStorage_PCPathHandling(t *testing.T) {
	mockStorage := NewMockStorage()
	ctx := context.Background()

	// Add PC with simple ID
	testPC := &actor.PCSpec{
		ID:    "test_hero",
		Name:  "Test Hero",
		Class: "paladin",
		HP:    30,
		MaxHP: 30,
	}
	mockStorage.AddPCSpec("test_hero", testPC)

	// Test various path formats
	testCases := []string{
		"test_hero",
		"test_hero.json",
		"../../data/pcs/test_hero.json",
		"/some/path/to/test_hero.json",
	}

	for _, path := range testCases {
		loaded, err := mockStorage.GetPCSpec(ctx, path)
		if err != nil {
			t.Errorf("Failed to get PC with path %q: %v", path, err)
			continue
		}

		if loaded.ID != "test_hero" {
			t.Errorf("For path %q, expected ID 'test_hero', got %v", path, loaded.ID)
		}
	}
}
