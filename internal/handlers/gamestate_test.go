package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/internal/services"
	"github.com/jwebster45206/roleplay-agent/pkg/state"
)

func TestGameStateHandler_Create(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	mockStorage := services.NewMockStorage()
	handler := NewGameStateHandler(mockStorage, logger)

	// Test creating a new game state
	req := httptest.NewRequest(http.MethodPost, "/gamestate", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Check status code
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rr.Code)
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
	if response.ID == uuid.Nil {
		t.Error("Expected non-nil game state ID")
	}
}

func TestGameStateHandler_Read(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	mockStorage := services.NewMockStorage()
	handler := NewGameStateHandler(mockStorage, logger)

	// Create a test game state
	testGS := state.NewGameState()
	if err := mockStorage.SaveGameState(context.Background(), testGS.ID, testGS); err != nil {
		t.Fatalf("Failed to save test game state: %v", err)
	}

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
			req := httptest.NewRequest(http.MethodGet, "/gamestate/"+tt.gameStateID, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectError {
				var response ErrorResponse
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

				if response.ID == uuid.Nil {
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

	mockStorage := services.NewMockStorage()
	handler := NewGameStateHandler(mockStorage, logger)

	// Create a test game state
	testGS := state.NewGameState()
	if err := mockStorage.SaveGameState(context.Background(), testGS.ID, testGS); err != nil {
		t.Fatalf("Failed to save test game state: %v", err)
	}

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
			expectedStatus: http.StatusNoContent,
			expectError:    false,
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
			req := httptest.NewRequest(http.MethodDelete, "/gamestate/"+tt.gameStateID, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectError {
				var response ErrorResponse
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

	mockStorage := services.NewMockStorage()
	handler := NewGameStateHandler(mockStorage, logger)

	// Test unsupported methods (DELETE is now supported)
	methods := []string{http.MethodPut, http.MethodPatch, http.MethodHead}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/gamestate", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for method %s, got %d", method, rr.Code)
			}

			var response ErrorResponse
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

	mockStorage := services.NewMockStorage()
	handler := NewGameStateHandler(mockStorage, logger)

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
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for %s without ID, got %d", tt.method, rr.Code)
			}

			var response ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Error == "" {
				t.Error("Expected error message for missing ID")
			}
		})
	}
}
