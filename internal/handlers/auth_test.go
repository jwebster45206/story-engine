package handlers

import (
	"context"
	"net/http"
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/internal/middleware"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

const (
	testAPIKeyA = "22222222-2222-4222-8222-222222222222"
	testAPIKeyB = "44444444-4444-4444-8444-444444444444"
)

func testKeyHash(key string) string {
	return config.HashKey(uuid.MustParse(key))
}

func testAuthConfig() *config.Config {
	return &config.Config{
		APIKeyHashes: []string{testKeyHash(testAPIKeyA), testKeyHash(testAPIKeyB)},
	}
}

func serveKey(h http.Handler, w http.ResponseWriter, r *http.Request, key string) {
	r.Header.Set("Authorization", "Bearer "+key)
	middleware.APIKey(testAuthConfig(), h).ServeHTTP(w, r)
}

func saveOwned(t *testing.T, store *storage.MockStorage, gs *state.GameState, key string) {
	t.Helper()
	if err := store.SaveGameState(context.Background(), gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOwnerKeyHash(context.Background(), gs.ID, testKeyHash(key)); err != nil {
		t.Fatal(err)
	}
}
