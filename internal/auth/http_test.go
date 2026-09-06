package auth

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCanAccess(t *testing.T) {
	a := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	b := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	p := Principal{ID: a}
	if !p.CanAccess(a) {
		t.Fatal("owner should match")
	}
	if p.CanAccess(b) {
		t.Fatal("other principal should not match")
	}
	if p.CanAccess(uuid.Nil()) {
		t.Fatal("nil owner should not match")
	}
	if (Principal{}).CanAccess(a) {
		t.Fatal("nil principal should not match")
	}
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
		id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate", nil)
		req = req.WithContext(WithPrincipal(req.Context(), Principal{ID: id}))
		rr := httptest.NewRecorder()
		p, ok := RequirePrincipal(rr, req, logger)
		if !ok || p.ID != id {
			t.Fatalf("ok=%v id=%v", ok, p.ID)
		}
	})
}

func TestAuthorizeGame(t *testing.T) {
	logger := discardLogger()
	store := storage.NewMockStorage()
	gs := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
	owner := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	other := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	if err := store.SaveGameState(t.Context(), gs.ID, gs); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOwner(t.Context(), gs.ID, owner); err != nil {
		t.Fatal(err)
	}

	withID := func(id uuid.UUID) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+gs.ID.String(), nil)
		return req.WithContext(WithPrincipal(req.Context(), Principal{ID: id}))
	}

	t.Run("owner", func(t *testing.T) {
		rr := httptest.NewRecorder()
		if !AuthorizeGame(rr, withID(owner), store, gs.ID, logger) {
			t.Fatal("owner should be allowed")
		}
	})

	t.Run("wrong owner", func(t *testing.T) {
		rr := httptest.NewRecorder()
		if AuthorizeGame(rr, withID(other), store, gs.ID, logger) {
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

	t.Run("missing owner", func(t *testing.T) {
		orphan := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
		if err := store.SaveGameState(t.Context(), orphan.ID, orphan); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+orphan.ID.String(), nil)
		req = req.WithContext(WithPrincipal(req.Context(), Principal{ID: owner}))
		rr := httptest.NewRecorder()
		if AuthorizeGame(rr, req, store, orphan.ID, logger) {
			t.Fatal("expected deny")
		}
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}
