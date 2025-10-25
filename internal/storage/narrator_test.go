package storage

import (
	"context"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func TestMockStorage_AddAndGetNarrator(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Add a test narrator
	testNarrator := &scenario.Narrator{
		ID:          "epic",
		Name:        "Epic Narrator",
		Description: "A dramatic and epic narrative voice",
		Prompts: []string{
			"You find yourself in a grand adventure...",
			"The stakes have never been higher...",
		},
	}

	mockStorage.AddNarrator("epic", testNarrator)

	// Get it back
	loaded, err := mockStorage.GetNarrator(ctx, "epic")
	if err != nil {
		t.Fatalf("Failed to get narrator: %v", err)
	}

	if loaded.ID != "epic" {
		t.Errorf("Expected ID 'epic', got %v", loaded.ID)
	}

	if loaded.Name != "Epic Narrator" {
		t.Errorf("Expected name 'Epic Narrator', got %v", loaded.Name)
	}

	if len(loaded.Prompts) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(loaded.Prompts))
	}
}

func TestMockStorage_GetNonExistentNarrator(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Try to get a non-existent narrator
	_, err := mockStorage.GetNarrator(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent narrator")
	}

	if err.Error() != "narrator not found" {
		t.Errorf("Expected 'narrator not found' error, got: %v", err)
	}
}

func TestMockStorage_GetNarratorEmptyID(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Get with empty ID should return nil, nil
	loaded, err := mockStorage.GetNarrator(ctx, "")
	if err != nil {
		t.Errorf("Expected no error for empty ID, got: %v", err)
	}

	if loaded != nil {
		t.Error("Expected nil narrator for empty ID")
	}
}

func TestMockStorage_ListNarrators(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Add multiple narrators
	mockStorage.AddNarrator("epic", &scenario.Narrator{
		ID:   "epic",
		Name: "Epic Narrator",
	})
	mockStorage.AddNarrator("mysterious", &scenario.Narrator{
		ID:   "mysterious",
		Name: "Mysterious Narrator",
	})
	mockStorage.AddNarrator("comedic", &scenario.Narrator{
		ID:   "comedic",
		Name: "Comedic Narrator",
	})

	// List them
	narrators, err := mockStorage.ListNarrators(ctx)
	if err != nil {
		t.Fatalf("Failed to list narrators: %v", err)
	}

	if len(narrators) != 3 {
		t.Errorf("Expected 3 narrators, got %d", len(narrators))
	}

	// Check that all IDs are present
	found := make(map[string]bool)
	for _, id := range narrators {
		found[id] = true
	}

	if !found["epic"] {
		t.Error("Expected to find 'epic' narrator")
	}
	if !found["mysterious"] {
		t.Error("Expected to find 'mysterious' narrator")
	}
	if !found["comedic"] {
		t.Error("Expected to find 'comedic' narrator")
	}
}

func TestMockStorage_ListNarratorsEmpty(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// List narrators when none exist
	narrators, err := mockStorage.ListNarrators(ctx)
	if err != nil {
		t.Fatalf("Failed to list narrators: %v", err)
	}

	if len(narrators) != 0 {
		t.Errorf("Expected 0 narrators, got %d", len(narrators))
	}
}
