package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/auth"
	"github.com/jwebster45206/story-engine/internal/httperror"
	"github.com/jwebster45206/story-engine/internal/llm"
	"github.com/jwebster45206/story-engine/internal/middleware"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func testCatalog() ProviderCatalog {
	return &stubCatalog{
		defaultName: "foo",
		infos: map[string]llm.ProviderInfo{
			"foo": {Name: "foo", DisplayName: "Foo", Vendor: "venice", Model: "foo_model"},
			"bar": {Name: "bar", DisplayName: "Bar", Vendor: "anthropic", Model: "bar_model"},
		},
	}
}

type stubCatalog struct {
	defaultName string
	infos       map[string]llm.ProviderInfo
}

func (s *stubCatalog) Default() string { return s.defaultName }
func (s *stubCatalog) Names() []string {
	names := make([]string, 0, len(s.infos))
	for n := range s.infos {
		names = append(names, n)
	}
	return names
}
func (s *stubCatalog) Info(name string) (llm.ProviderInfo, bool) {
	info, ok := s.infos[name]
	return info, ok
}

var (
	testOwnerID    = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	testPrincipalA = testOwnerID
	testPrincipalB = uuid.MustParse("44444444-4444-4444-8444-444444444444")
)

func withTestPrincipal(r *http.Request) *http.Request {
	return r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{ID: testOwnerID}))
}

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

// saveOwned writes a gamestate and stamps its owner so tests can skip POST /gamestate.
func saveOwned(t *testing.T, ctx context.Context, store *storage.MockStorage, gs *state.GameState, ownerID uuid.UUID) {
	t.Helper()
	gs.PrincipalID = ownerID
	if err := store.CreateGameState(ctx, gs.ID, gs); err != nil {
		t.Fatal(err)
	}
}

func TestGameStateHandler_Create(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	mockStorage := storage.NewMockStorage()

	// Add the test scenario
	mockStorage.AddScenario("foo_scenario.json", &scenario.Scenario{
		Name:            "Test Scenario",
		FileName:        "foo_scenario.json",
		Story:           "A test scenario",
		OpeningPrompt:   "Welcome to the test!",
		OpeningLocation: "start",
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	})

	handler := NewGameStateHandler(logger, testCatalog(), mockStorage)

	// Test creating a new game state
	reqBody := `{"scenario":"foo_scenario.json"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/gamestate", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json") // This was missing!
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, withTestPrincipal(req))

	// Check status code
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Response body: %s", rr.Code, rr.Body.String())
	}

	// Check content type
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
	}

	// Parse response
	var response state.GameState
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate response
	if response.ID == uuid.Nil() {
		t.Error("Expected non-nil game state ID")
	}
	if response.Rules != state.RulesStrict {
		t.Errorf("Expected default rules %q, got %q", state.RulesStrict, response.Rules)
	}
	if response.Temperature != state.DefaultTemperature {
		t.Errorf("Expected default temperature %f, got %f", state.DefaultTemperature, response.Temperature)
	}
	if response.Provider != "foo" {
		t.Errorf("Expected default provider foo, got %q", response.Provider)
	}
	if response.ModelName != "foo_model" {
		t.Errorf("Expected stamped model foo_model, got %q", response.ModelName)
	}
	if response.PrincipalID != testOwnerID {
		t.Errorf("Expected principal %v, got %v", testOwnerID, response.PrincipalID)
	}
}

func TestGameStateHandler_CreateWithOverrides(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	mockStorage := storage.NewMockStorage()

	// Add the test scenario that tests reference
	mockStorage.AddScenario("foo_scenario.json", &scenario.Scenario{
		Name:            "Test Scenario",
		FileName:        "foo_scenario.json",
		Story:           "A test scenario",
		OpeningPrompt:   "Welcome to the test!",
		OpeningLocation: "start",
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	})

	handler := NewGameStateHandler(logger, testCatalog(), mockStorage)

	tests := []struct {
		name            string
		requestBody     string
		expectedStatus  int
		checkNarratorID string
		checkPCID       string
	}{
		{
			name:           "with narrator override",
			requestBody:    `{"scenario":"foo_scenario.json","narrator":{"id":"epic"}}`,
			expectedStatus: http.StatusCreated,
			// Note: Will use fallback since 'epic' doesn't exist in test env
		},
		{
			name:           "with pc override",
			requestBody:    `{"scenario":"foo_scenario.json","pc":{"id":"custom_hero"}}`,
			expectedStatus: http.StatusCreated,
			// Note: Will use fallback since 'custom_hero' doesn't exist in test env
		},
		{
			name:           "with both overrides",
			requestBody:    `{"scenario":"foo_scenario.json","narrator":{"id":"epic"},"pc":{"id":"custom_hero"}}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "with empty overrides (should use defaults)",
			requestBody:    `{"scenario":"foo_scenario.json","narrator":{"id":""},"pc":{"id":""}}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing scenario field",
			requestBody:    `{"narrator":{"id":"epic"}}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unknown provider",
			requestBody:    `{"scenario":"foo_scenario.json","provider":"nope"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "explicit provider stamps model",
			requestBody:    `{"scenario":"foo_scenario.json","provider":"bar","model_name":"client-ignored"}`,
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/gamestate", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, withTestPrincipal(req))

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Response body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.expectedStatus == http.StatusCreated {
				// Parse response
				var response state.GameState
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Validate response
				if response.ID == uuid.Nil() {
					t.Error("Expected non-nil game state ID")
				}

				if strings.Contains(tt.requestBody, `"provider":"bar"`) {
					if response.Provider != "bar" || response.ModelName != "bar_model" {
						t.Errorf("provider/model = %q/%q, want bar/bar_model", response.Provider, response.ModelName)
					}
				}

				// Check that narrator is embedded if specified
				if tt.checkNarratorID != "" {
					if response.Narrator == nil {
						t.Errorf("Expected narrator to be embedded, got nil")
					} else if response.Narrator.ID != tt.checkNarratorID {
						t.Errorf("Expected narrator ID %s, got %s", tt.checkNarratorID, response.Narrator.ID)
					}
				}

				// Verify the gamestate was saved
				retrievedGS, err := mockStorage.LoadGameState(context.Background(), response.ID)
				if err != nil {
					t.Errorf("Failed to retrieve saved game state: %v", err)
				}
				if retrievedGS == nil {
					t.Error("Expected saved game state to exist in storage")
				}
			} else {
				// Should be an error response
				var response httperror.Response
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}
				if response.Error == "" {
					t.Error("Expected error message in response")
				}
			}
		})
	}
}

func TestGameStateHandler_Read(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	mockStorage := storage.NewMockStorage()
	handler := NewGameStateHandler(logger, testCatalog(), mockStorage)

	// Create a test game state (nil narrator is fine for tests)
	testGS := state.NewGameState("FooScenario", nil, "test-provider", "foo_model")
	saveOwned(t, t.Context(), mockStorage, testGS, testOwnerID)

	tests := []struct {
		name           string
		gameStateID    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "valid game state ID",
			gameStateID:    testGS.ID.String(),
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "non-existent game state ID",
			gameStateID:    uuid.New().String(),
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "invalid game state ID format",
			gameStateID:    "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/gamestate/"+tt.gameStateID, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, withTestPrincipal(req))

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectError {
				var response httperror.Response
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if response.Error == "" {
					t.Error("Expected error in response")
				}
			} else {
				var response state.GameState
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.ID == uuid.Nil() {
					t.Error("Expected valid game state ID in response")
				}
			}
		})
	}
}

func TestGameStateHandler_Delete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	mockStorage := storage.NewMockStorage()
	handler := NewGameStateHandler(logger, testCatalog(), mockStorage)

	// Create a test game state (nil narrator is fine for tests)
	testGS := state.NewGameState("FooScenario", nil, "test-provider", "foo_model")
	saveOwned(t, t.Context(), mockStorage, testGS, testOwnerID)

	tests := []struct {
		name           string
		gameStateID    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "valid delete",
			gameStateID:    testGS.ID.String(),
			expectedStatus: http.StatusNoContent,
			expectError:    false,
		},
		{
			name:           "non-existent game state",
			gameStateID:    uuid.New().String(),
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "invalid game state ID format",
			gameStateID:    "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/v1/gamestate/"+tt.gameStateID, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, withTestPrincipal(req))

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectError {
				var response httperror.Response
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if response.Error == "" {
					t.Error("Expected error in response")
				}
			} else {
				// For successful delete, we expect no content (status 204)
				if rr.Body.Len() != 0 {
					t.Error("Expected empty response body for successful delete")
				}
			}
		})
	}
}

func TestGameStateHandler_MethodNotAllowed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	mockStorage := storage.NewMockStorage()
	handler := NewGameStateHandler(logger, testCatalog(), mockStorage)

	// Test unsupported methods
	methods := []string{http.MethodPut, http.MethodHead}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/v1/gamestate", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for method %s, got %d", method, rr.Code)
			}

			var response httperror.Response
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Error == "" {
				t.Error("Expected error message for unsupported method")
			}
		})
	}
}

func TestGameStateHandler_MissingID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	mockStorage := storage.NewMockStorage()
	handler := NewGameStateHandler(logger, testCatalog(), mockStorage)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "GET without ID",
			method: http.MethodGet,
			path:   "/gamestate",
		},
		{
			name:   "DELETE without ID",
			method: http.MethodDelete,
			path:   "/gamestate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Update path to use v1 prefix
			v1Path := strings.Replace(tt.path, "/gamestate", "/v1/gamestate", 1)
			req := httptest.NewRequest(tt.method, v1Path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for %s without ID, got %d", tt.method, rr.Code)
			}

			var response httperror.Response
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Error == "" {
				t.Error("Expected error message for missing ID")
			}
		})
	}
}

func TestGameStateHandler_CreateRulesAndTemperature(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
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

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		wantRules      state.RulesMode
		wantTemp       float64
	}{
		{
			name:           "relaxed rules and custom temperature",
			requestBody:    `{"scenario":"foo_scenario.json","rules":"relaxed","temperature":0.8}`,
			expectedStatus: http.StatusCreated,
			wantRules:      state.RulesRelaxed,
			wantTemp:       0.8,
		},
		{
			name:           "invalid rules",
			requestBody:    `{"scenario":"foo_scenario.json","rules":"chaotic"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "temperature too high",
			requestBody:    `{"scenario":"foo_scenario.json","temperature":1.5}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "temperature too low",
			requestBody:    `{"scenario":"foo_scenario.json","temperature":-0.1}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/gamestate", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, withTestPrincipal(req))

			if rr.Code != tt.expectedStatus {
				t.Fatalf("Expected status %d, got %d. Body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
			if tt.expectedStatus != http.StatusCreated {
				return
			}
			var response state.GameState
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			if response.Rules != tt.wantRules {
				t.Errorf("Rules = %q, want %q", response.Rules, tt.wantRules)
			}
			if response.Temperature != tt.wantTemp {
				t.Errorf("Temperature = %f, want %f", response.Temperature, tt.wantTemp)
			}
		})
	}
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
	if created.PrincipalID != testPrincipalA {
		t.Fatalf("principal = %v, want %v", created.PrincipalID, testPrincipalA)
	}
	ownerID, err := mockStorage.GetOwner(t.Context(), created.ID)
	if err != nil || ownerID != testPrincipalA {
		t.Fatalf("ownerID = %v err=%v", ownerID, err)
	}

	orphan := state.NewGameState("foo_scenario.json", nil, "foo", "foo_model")
	if err := mockStorage.UpdateGameState(t.Context(), orphan.ID, orphan); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		token  string
		want   int
	}{
		{name: "owner reads", method: http.MethodGet, path: "/v1/gamestate/" + created.ID.String(), token: tokenA, want: http.StatusOK},
		{name: "other principal 403", method: http.MethodGet, path: "/v1/gamestate/" + created.ID.String(), token: tokenB, want: http.StatusForbidden},
		{name: "missing owner sidecar 404", method: http.MethodGet, path: "/v1/gamestate/" + orphan.ID.String(), token: tokenA, want: http.StatusNotFound},
		{name: "owner patches", method: http.MethodPatch, path: "/v1/gamestate/" + created.ID.String(), body: `{"location":"forest"}`, token: tokenA, want: http.StatusOK},
		{name: "other principal patch 403", method: http.MethodPatch, path: "/v1/gamestate/" + created.ID.String(), body: `{"location":"cave"}`, token: tokenB, want: http.StatusForbidden},
		{name: "other principal delete 403", method: http.MethodDelete, path: "/v1/gamestate/" + created.ID.String(), token: tokenB, want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *strings.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			serveJWT(t, handler, pub, rr, req, tt.token)
			if rr.Code != tt.want {
				t.Fatalf("status = %d, want %d body=%s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}

	ownerID, err = mockStorage.GetOwner(t.Context(), created.ID)
	if err != nil || ownerID != testPrincipalA {
		t.Fatalf("ownerID after patch = %v err=%v", ownerID, err)
	}
}
