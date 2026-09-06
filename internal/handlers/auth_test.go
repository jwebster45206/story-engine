package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/auth"
	"github.com/jwebster45206/story-engine/internal/middleware"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

var (
	testPrincipalA = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	testPrincipalB = uuid.MustParse("44444444-4444-4444-8444-444444444444")
)

func testKeyPair(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv, &priv.PublicKey
}

func serveJWT(t *testing.T, h http.Handler, pub *ecdsa.PublicKey, w http.ResponseWriter, r *http.Request, raw string) {
	t.Helper()
	r.Header.Set("Authorization", "Bearer "+raw)
	middleware.JWT(pub, h).ServeHTTP(w, r)
}

func tokenFor(t *testing.T, priv *ecdsa.PrivateKey, id uuid.UUID) string {
	t.Helper()
	raw, err := auth.Token(priv, id)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestGameStateOwnership(t *testing.T) {
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
	handler := NewGameStateHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), testCatalog(), mockStorage)
	priv, pub := testKeyPair(t)
	tokenA := tokenFor(t, priv, testPrincipalA)
	tokenB := tokenFor(t, priv, testPrincipalB)

	req := httptest.NewRequest(http.MethodPost, "/v1/gamestate", strings.NewReader(`{"scenario":"foo_scenario.json"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	serveJWT(t, handler, pub, rr, req, tokenA)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rr.Code, rr.Body.String())
	}
	var created state.GameState
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	owner, found, err := mockStorage.GetOwner(t.Context(), created.ID)
	if err != nil || !found || owner != testPrincipalA {
		t.Fatalf("owner = %v found=%v err=%v", owner, found, err)
	}

	t.Run("owner reads", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+created.ID.String(), nil)
		rr := httptest.NewRecorder()
		serveJWT(t, handler, pub, rr, req, tokenA)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("other principal 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+created.ID.String(), nil)
		rr := httptest.NewRecorder()
		serveJWT(t, handler, pub, rr, req, tokenB)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("missing owner sidecar 404", func(t *testing.T) {
		orphan := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
		if err := mockStorage.SaveGameState(t.Context(), orphan.ID, orphan); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+orphan.ID.String(), nil)
		rr := httptest.NewRecorder()
		serveJWT(t, handler, pub, rr, req, tokenA)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("patch does not change owner", func(t *testing.T) {
		body := `{"location":"forest"}`
		req := httptest.NewRequest(http.MethodPatch, "/v1/gamestate/"+created.ID.String(), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		serveJWT(t, handler, pub, rr, req, tokenA)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
		owner, found, err := mockStorage.GetOwner(t.Context(), created.ID)
		if err != nil || !found || owner != testPrincipalA {
			t.Fatalf("owner = %v found=%v err=%v", owner, found, err)
		}
	})
}
