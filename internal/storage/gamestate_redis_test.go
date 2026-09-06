package storage

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/alicebob/miniredis/v2"
	"github.com/jwebster45206/story-engine/pkg/state"
)

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

func TestRedisStorage_OwnerSyncedTTL(t *testing.T) {
	store, mr := newTestRedisStorage(t)
	ctx := context.Background()
	owner := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	if err := store.SaveGameState(ctx, gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOwner(ctx, gs.ID, owner); err != nil {
		t.Fatal(err)
	}

	got, found, err := store.GetOwner(ctx, gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got != owner {
		t.Fatalf("found=%v owner=%v", found, got)
	}

	blobTTL := mr.TTL(gamestateKey(gs.ID))
	ownerTTL := mr.TTL(gamestateOwnerKey(gs.ID))
	if blobTTL != gameStateTTL {
		t.Fatalf("gamestate TTL = %v, want %v", blobTTL, gameStateTTL)
	}
	if ownerTTL != blobTTL {
		t.Fatalf("owner TTL = %v, blob TTL = %v", ownerTTL, blobTTL)
	}

	mr.FastForward(gameStateTTL + time.Second)
	_, found, err = store.GetOwner(ctx, gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("owner key should expire with gamestate")
	}
	loaded, err := store.LoadGameState(ctx, gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatal("gamestate blob should expire with owner key")
	}
}

func TestRedisStorage_SaveDoesNotChangeOwner(t *testing.T) {
	store, _ := newTestRedisStorage(t)
	ctx := context.Background()
	owner := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	if err := store.SaveGameState(ctx, gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOwner(ctx, gs.ID, owner); err != nil {
		t.Fatal(err)
	}

	gs.Location = "forest"
	if err := store.SaveGameState(ctx, gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.GetOwner(ctx, gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got != owner {
		t.Fatalf("found=%v owner=%v", found, got)
	}
}

func TestRedisStorage_DeleteRemovesOwner(t *testing.T) {
	store, _ := newTestRedisStorage(t)
	ctx := context.Background()
	owner := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	if err := store.SaveGameState(ctx, gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOwner(ctx, gs.ID, owner); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteGameState(ctx, gs.ID); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.GetOwner(ctx, gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found || got != uuid.Nil() {
		t.Fatalf("found=%v owner=%v after delete", found, got)
	}
}

func TestRedisStorage_GetOwnerMissing(t *testing.T) {
	store, _ := newTestRedisStorage(t)
	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	got, found, err := store.GetOwner(context.Background(), gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found || got != uuid.Nil() {
		t.Fatalf("found=%v owner=%v, want missing", found, got)
	}
}
