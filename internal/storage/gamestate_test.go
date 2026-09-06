package storage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/alicebob/miniredis/v2"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func TestMockStorage_SaveAndLoadGameState(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	ctx := context.Background()

	// Create a test gamestate
	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	gs.Location = "tavern"
	gs.Inventory = []string{"sword", "shield"}

	// Save it
	err := mockStorage.UpdateGameState(ctx, gs.ID, gs)
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
		return
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
	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	err := mockStorage.UpdateGameState(ctx, gs.ID, gs)
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
	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	gs.Location = "start"
	err := mockStorage.UpdateGameState(ctx, gs.ID, gs)
	if err != nil {
		t.Fatalf("Failed to save initial gamestate: %v", err)
	}

	// Update it
	gs.Location = "forest"
	gs.Inventory = append(gs.Inventory, "potion")
	time.Sleep(10 * time.Millisecond) // Ensure UpdatedAt will be different

	err = mockStorage.UpdateGameState(ctx, gs.ID, gs)
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

func newTestRedisStorage(t *testing.T) (*RedisStorage, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := NewRedisStorage(mr.Addr(), t.TempDir(), logger)
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store, mr
}

func TestRedisStorage_Owner(t *testing.T) {
	store, mr := newTestRedisStorage(t)
	ctx := t.Context()
	ownerID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")

	if err := store.CreateGameState(ctx, uuid.Nil(), gs, ownerID); err == nil {
		t.Fatal("CreateGameState with nil id should fail")
	}
	if err := store.CreateGameState(ctx, gs.ID, gs, uuid.Nil()); err == nil {
		t.Fatal("CreateGameState with nil ownerID should fail")
	}

	if err := store.CreateGameState(ctx, gs.ID, gs, ownerID); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetOwner(ctx, gs.ID)
	if err != nil || got != ownerID {
		t.Fatalf("GetOwner = %v err=%v", got, err)
	}

	gs.Location = "forest"
	if err := store.UpdateGameState(ctx, gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetOwner(ctx, gs.ID)
	if err != nil || got != ownerID {
		t.Fatalf("UpdateGameState changed owner: %v err=%v", got, err)
	}

	blobTTL := mr.TTL(gamestateKey(gs.ID))
	ownerTTL := mr.TTL(gamestateOwnerKey(gs.ID))
	if blobTTL != gameStateTTL {
		t.Fatalf("gamestate TTL = %v, want %v", blobTTL, gameStateTTL)
	}
	if ownerTTL != blobTTL {
		t.Fatalf("owner TTL = %v, blob TTL = %v", ownerTTL, blobTTL)
	}

	if err := store.DeleteGameState(ctx, gs.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.GetOwner(ctx, gs.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("after delete GetOwner err = %v, want ErrNotFound", err)
	}

	missing := uuid.New()
	_, err = store.GetOwner(ctx, missing)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing GetOwner err = %v, want ErrNotFound", err)
	}

	gs2 := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	if err := store.CreateGameState(ctx, gs2.ID, gs2, ownerID); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(gameStateTTL + time.Second)
	_, err = store.GetOwner(ctx, gs2.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expired GetOwner err = %v, want ErrNotFound", err)
	}
	loaded, err := store.LoadGameState(ctx, gs2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatal("gamestate blob should expire with owner key")
	}
}
