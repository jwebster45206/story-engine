package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// MockStorage implements the Storage interface for testing
type MockStorage struct {
	scenarios map[string]*scenario.Scenario
	shouldErr bool
}

func (m *MockStorage) GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error) {
	if m.shouldErr {
		return nil, errors.New("storage error")
	}

	if filename == "notfound.json" {
		return nil, errors.New("scenario not found: notfound.json")
	}

	if s, exists := m.scenarios[filename]; exists {
		return s, nil
	}

	return nil, errors.New("scenario not found: " + filename)
}

// Mock other Storage interface methods (not used in this test)
func (m *MockStorage) SaveGameState(ctx context.Context, uuid uuid.UUID, gamestate *state.GameState) error {
	return nil
}
func (m *MockStorage) LoadGameState(ctx context.Context, uuid uuid.UUID) (*state.GameState, error) {
	return nil, nil
}
func (m *MockStorage) DeleteGameState(ctx context.Context, uuid uuid.UUID) error    { return nil }
func (m *MockStorage) ListScenarios(ctx context.Context) (map[string]string, error) { return nil, nil }
func (m *MockStorage) Ping(ctx context.Context) error                               { return nil }
func (m *MockStorage) Close() error                                                 { return nil }

func TestScenarioHandler_ServeHTTP(t *testing.T) {
	// Create a test logger that discards output
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create mock storage with test data
	mockStorage := &MockStorage{
		scenarios: map[string]*scenario.Scenario{
			"pirate.json": {
				Name:            "Pirate Adventure",
				FileName:        "pirate.json",
				Story:           "A swashbuckling adventure on the high seas",
				OpeningPrompt:   "Welcome to the pirate adventure!",
				OpeningLocation: "ship_deck",
				Locations: map[string]scenario.Location{
					"ship_deck": {
						Name:        "ship_deck",
						Description: "You are on the deck of a pirate ship",
						Exits:       map[string]string{"north": "captain_cabin"},
					},
					"captain_cabin": {
						Name:        "captain_cabin",
						Description: "The private cabin of the pirate captain.",
					},
				},
				NPCs: map[string]scenario.NPC{
					"Captain Blackbeard": {
						Name:        "Captain Blackbeard",
						Type:        "captain",
						Disposition: "neutral",
						Description: "A fearsome pirate captain",
						IsImportant: true,
						Location:    "ship_deck",
					},
				},
				Inventory:        []string{"sword", "compass", "map"},
				OpeningInventory: []string{"sword"},
			},
		},
	}

	handler := NewScenarioHandler(logger, mockStorage)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
		shouldErr      bool
	}{
		{
			name:           "Valid scenario request",
			method:         "GET",
			path:           "/v1/scenarios/pirate.json",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"name":"Pirate Adventure","file_name":"pirate.json","story":"A swashbuckling adventure on the high seas"`,
		},
		{
			name:           "Missing filename",
			method:         "GET",
			path:           "/v1/scenarios/",
			expectedStatus: http.StatusOK,
			expectedBody:   "null",
		},
		{
			name:           "Invalid filename with path traversal",
			method:         "GET",
			path:           "/v1/scenarios/../etc/passwd",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid filename",
		},
		{
			name:           "Invalid filename with slash",
			method:         "GET",
			path:           "/v1/scenarios/subdir/file.json",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid filename",
		},
		{
			name:           "Scenario not found",
			method:         "GET",
			path:           "/v1/scenarios/notfound.json",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Scenario not found",
		},
		{
			name:           "Method not allowed",
			method:         "POST",
			path:           "/v1/scenarios/pirate.json",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage.shouldErr = tt.shouldErr

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" {
				body := w.Body.String()
				if !contains(body, tt.expectedBody) {
					t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, body)
				}
			}

			// For successful requests, verify JSON response
			if tt.expectedStatus == http.StatusOK && tt.name != "Missing filename" {
				var response scenario.Scenario
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if response.Name != "Pirate Adventure" {
					t.Errorf("Expected scenario name 'Pirate Adventure', got %q", response.Name)
				}
			}
		})
	}
}

func TestScenarioHandler_StorageError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockStorage := &MockStorage{
		shouldErr: true,
	}

	handler := NewScenarioHandler(logger, mockStorage)

	req := httptest.NewRequest("GET", "/v1/scenarios/pirate.json", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	if !contains(w.Body.String(), "Failed to retrieve scenario") {
		t.Errorf("Expected error message about failed retrieval, got %q", w.Body.String())
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
