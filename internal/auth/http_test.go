package auth

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRequirePrincipal(t *testing.T) {
	logger := discardLogger()

	t.Run("missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate", nil)
		rr := httptest.NewRecorder()
		if _, ok := RequirePrincipal(rr, req, logger); ok {
			t.Fatal("expected false")
		}
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate", nil)
		req = req.WithContext(WithPrincipal(req.Context(), Principal{KeyHash: "h"}))
		rr := httptest.NewRecorder()
		p, ok := RequirePrincipal(rr, req, logger)
		if !ok || p.KeyHash != "h" {
			t.Fatalf("ok=%v hash=%q", ok, p.KeyHash)
		}
	})
}

func TestAuthorizeGame(t *testing.T) {
	logger := discardLogger()
	store := storage.NewMockStorage()
	gs := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
	if err := store.SaveGameState(t.Context(), gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOwnerKeyHash(t.Context(), gs.ID, "owner-a"); err != nil {
		t.Fatal(err)
	}

	withHash := func(hash string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+gs.ID.String(), nil)
		return req.WithContext(WithPrincipal(req.Context(), Principal{KeyHash: hash}))
	}

	t.Run("owner", func(t *testing.T) {
		rr := httptest.NewRecorder()
		if !AuthorizeGame(rr, withHash("owner-a"), store, gs.ID, logger) {
			t.Fatal("owner should be allowed")
		}
	})

	t.Run("wrong owner", func(t *testing.T) {
		rr := httptest.NewRecorder()
		if AuthorizeGame(rr, withHash("owner-b"), store, gs.ID, logger) {
			t.Fatal("expected deny")
		}
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
		var body map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["error"] != "Game state not found" {
			t.Fatalf("error = %q", body["error"])
		}
	})
}
