package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func TestGameStateOwnership(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockStorage := storage.NewMockStorage()
	mockStorage.AddScenario("foo_scenario.json", &scenario.Scenario{
		Name:            "Test Scenario",
		FileName:        "foo_scenario.json",
		Story:           "A test scenario",
		OpeningPrompt:   "Welcome!",
		OpeningLocation: "start",
		Locations: map[string]scenario.Location{
			"start": {Name: "start", Description: "Starting location"},
		},
	})
	handler := NewGameStateHandler(logger, testCatalog(), mockStorage)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/gamestate", strings.NewReader(`{"scenario":"foo_scenario.json"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	handler.ServeHTTP(createRR, withAPIKey(createReq, testOwnerHashA))
	if createRR.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createRR.Code, createRR.Body.String())
	}
	rawCreate := createRR.Body.String()
	if strings.Contains(rawCreate, "owner_key_hash") {
		t.Fatal("raw create body contains owner_key_hash")
	}

	var created state.GameState
	if err := json.NewDecoder(strings.NewReader(rawCreate)).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.OwnerKeyHash != "" {
		t.Fatalf("HTTP response leaked owner_key_hash %q", created.OwnerKeyHash)
	}

	stored, err := mockStorage.LoadGameState(context.Background(), created.ID)
	if err != nil || stored == nil {
		t.Fatalf("stored gamestate: %v", err)
	}
	if stored.OwnerKeyHash != testOwnerHashA {
		t.Fatalf("stored hash = %q, want %q", stored.OwnerKeyHash, testOwnerHashA)
	}
	ownerHash, found, err := mockStorage.GetOwnerKeyHash(context.Background(), created.ID)
	if err != nil || !found || ownerHash != testOwnerHashA {
		t.Fatalf("owner key hash found=%v hash=%q err=%v", found, ownerHash, err)
	}

	t.Run("owner can get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+created.ID.String(), nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, withAPIKey(req, testOwnerHashA))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
		raw := rr.Body.String()
		if strings.Contains(raw, "owner_key_hash") {
			t.Fatal("GET leaked owner_key_hash")
		}
	})

	t.Run("other api_key gets 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+created.ID.String(), nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, withAPIKey(req, testOwnerHashB))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("admin can get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+created.ID.String(), nil)
		rr := httptest.NewRecorder()
		serveAdmin(handler, rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("patch cannot change owner", func(t *testing.T) {
		body := `{"owner_key_hash":"` + testOwnerHashB + `","user_location":"start"}`
		req := httptest.NewRequest(http.MethodPatch, "/v1/gamestate/"+created.ID.String(), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, withAPIKey(req, testOwnerHashA))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
		after, err := mockStorage.LoadGameState(context.Background(), created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.OwnerKeyHash != testOwnerHashA {
			t.Fatalf("owner hash changed to %q", after.OwnerKeyHash)
		}
	})

	t.Run("create ignores client owner_key_hash", func(t *testing.T) {
		body := `{"scenario":"foo_scenario.json","owner_key_hash":"` + testOwnerHashB + `"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/gamestate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, withAPIKey(req, testOwnerHashA))
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d", rr.Code)
		}
		var gs state.GameState
		if err := json.NewDecoder(rr.Body).Decode(&gs); err != nil {
			t.Fatal(err)
		}
		storedGS, err := mockStorage.LoadGameState(context.Background(), gs.ID)
		if err != nil {
			t.Fatal(err)
		}
		if storedGS.OwnerKeyHash != testOwnerHashA {
			t.Fatalf("hash = %q, want creator hash", storedGS.OwnerKeyHash)
		}
	})

	t.Run("empty owner hash is admin-only", func(t *testing.T) {
		gs := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
		if err := mockStorage.SaveGameState(context.Background(), gs.ID, gs); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+gs.ID.String(), nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, withAPIKey(req, testOwnerHashA))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("api_key status = %d, want 404", rr.Code)
		}
		req2 := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+gs.ID.String(), nil)
		rr2 := httptest.NewRecorder()
		serveAdmin(handler, rr2, req2)
		if rr2.Code != http.StatusOK {
			t.Fatalf("admin status = %d", rr2.Code)
		}
	})
}
