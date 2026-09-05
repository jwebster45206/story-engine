package storage

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

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

func TestRedisStorage_OwnerKeySyncedTTL(t *testing.T) {
	store, mr := newTestRedisStorage(t)
	ctx := context.Background()

	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	gs.OwnerKeyHash = "owner-hash-abc"
	if err := store.SaveGameState(ctx, gs.ID, gs); err != nil {
		t.Fatal(err)
	}

	hash, found, err := store.GetOwnerKeyHash(ctx, gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || hash != "owner-hash-abc" {
		t.Fatalf("found=%v hash=%q", found, hash)
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
	_, found, err = store.GetOwnerKeyHash(ctx, gs.ID)
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

func TestRedisStorage_DeleteRemovesOwnerKey(t *testing.T) {
	store, _ := newTestRedisStorage(t)
	ctx := context.Background()

	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	gs.OwnerKeyHash = "owner-hash-abc"
	if err := store.SaveGameState(ctx, gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteGameState(ctx, gs.ID); err != nil {
		t.Fatal(err)
	}
	hash, found, err := store.GetOwnerKeyHash(ctx, gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found || hash != "" {
		t.Fatalf("found=%v hash=%q after delete", found, hash)
	}
}

func TestRedisStorage_GetOwnerKeyHashMissing(t *testing.T) {
	store, _ := newTestRedisStorage(t)
	gs := state.NewGameState("test_scenario.json", nil, "test-provider", "test_model")
	hash, found, err := store.GetOwnerKeyHash(context.Background(), gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found || hash != "" {
		t.Fatalf("found=%v hash=%q, want missing", found, hash)
	}
}
