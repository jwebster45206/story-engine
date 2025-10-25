package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func TestMockStorage_SaveAndLoadGameState(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Create a test gamestate
	gs := state.NewGameState("test_scenario.json", nil, "test_model")
	gs.Location = "tavern"
	gs.Inventory = []string{"sword", "shield"}

	// Save it
	err := mockStorage.SaveGameState(ctx, gs.ID, gs)
	if err != nil {
		t.Fatalf("Failed to save gamestate: %v", err)
	}

	// Load it back
	loaded, err := mockStorage.LoadGameState(ctx, gs.ID)
	if err != nil {
		t.Fatalf("Failed to load gamestate: %v", err)
	}

	if loaded == nil {
		t.Fatal("Expected non-nil gamestate")
	}

	if loaded.ID != gs.ID {
		t.Errorf("Expected ID %v, got %v", gs.ID, loaded.ID)
	}

	if loaded.Location != "tavern" {
		t.Errorf("Expected location 'tavern', got %v", loaded.Location)
	}

	if len(loaded.Inventory) != 2 {
		t.Errorf("Expected 2 inventory items, got %d", len(loaded.Inventory))
	}
}

func TestMockStorage_LoadNonExistentGameState(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Try to load a non-existent gamestate
	id := uuid.New()
	loaded, err := mockStorage.LoadGameState(ctx, id)
	if err != nil {
		t.Fatalf("Expected no error for non-existent gamestate, got: %v", err)
	}

	if loaded != nil {
		t.Error("Expected nil for non-existent gamestate")
	}
}

func TestMockStorage_DeleteGameState(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Create and save a gamestate
	gs := state.NewGameState("test_scenario.json", nil, "test_model")
	err := mockStorage.SaveGameState(ctx, gs.ID, gs)
	if err != nil {
		t.Fatalf("Failed to save gamestate: %v", err)
	}

	// Verify it exists
	loaded, err := mockStorage.LoadGameState(ctx, gs.ID)
	if err != nil || loaded == nil {
		t.Fatal("Gamestate should exist before deletion")
	}

	// Delete it
	err = mockStorage.DeleteGameState(ctx, gs.ID)
	if err != nil {
		t.Fatalf("Failed to delete gamestate: %v", err)
	}

	// Verify it's gone
	loaded, err = mockStorage.LoadGameState(ctx, gs.ID)
	if err != nil {
		t.Fatalf("Unexpected error after deletion: %v", err)
	}
	if loaded != nil {
		t.Error("Gamestate should be nil after deletion")
	}
}

func TestMockStorage_UpdateGameState(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Create and save initial gamestate
	gs := state.NewGameState("test_scenario.json", nil, "test_model")
	gs.Location = "start"
	err := mockStorage.SaveGameState(ctx, gs.ID, gs)
	if err != nil {
		t.Fatalf("Failed to save initial gamestate: %v", err)
	}

	// Update it
	gs.Location = "forest"
	gs.Inventory = append(gs.Inventory, "potion")
	time.Sleep(10 * time.Millisecond) // Ensure UpdatedAt will be different

	err = mockStorage.SaveGameState(ctx, gs.ID, gs)
	if err != nil {
		t.Fatalf("Failed to update gamestate: %v", err)
	}

	// Load and verify update
	loaded, err := mockStorage.LoadGameState(ctx, gs.ID)
	if err != nil || loaded == nil {
		t.Fatal("Failed to load updated gamestate")
	}

	if loaded.Location != "forest" {
		t.Errorf("Expected location 'forest', got %v", loaded.Location)
	}

	if len(loaded.Inventory) != 1 || loaded.Inventory[0] != "potion" {
		t.Errorf("Expected inventory with 'potion', got %v", loaded.Inventory)
	}
}
