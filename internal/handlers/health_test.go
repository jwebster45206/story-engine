package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func TestHealthHandler_ServeHTTP(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	tests := []struct {
		name            string
		setupStorage    func() storage.Storage
		expectedStatus  int
		expectedHealth  string
		expectedStorage string
	}{
		{
			name: "all healthy",
			setupStorage: func() storage.Storage {
				mockStorage := storage.NewMockStorage()
				mockStorage.SetPingSuccess() // Storage is healthy
				return mockStorage
			},
			expectedStatus:  http.StatusOK,
			expectedHealth:  "healthy",
			expectedStorage: "healthy",
		},
		{
			name: "unhealthy storage",
			setupStorage: func() storage.Storage {
				mockStorage := storage.NewMockStorage()
				mockStorage.SetPingError(errors.New("connection failed")) // Storage is unhealthy
				return mockStorage
			},
			expectedStatus:  http.StatusServiceUnavailable,
			expectedHealth:  "degraded",
			expectedStorage: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := tt.setupStorage()
			// Create a mock LLM service for the handler (even though we don't use it in health check)
			mockLLM := services.NewMockLLMAPI()
			handler := NewHealthHandler(logger, storage, mockLLM)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check content type
			if rr.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
			}

			// Parse response
			var response HealthResponse
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Check overall status
			if response.Status != tt.expectedHealth {
				t.Errorf("Expected status '%s', got '%s'", tt.expectedHealth, response.Status)
			}

			// Check service name
			if response.Service != "story-engine" {
				t.Errorf("Expected service 'story-engine', got '%s'", response.Service)
			}

			// Check storage component status
			storageComponent, exists := response.Components["storage"]
			if !exists {
				t.Error("Expected storage component in response")
			} else if storageComponent != tt.expectedStorage {
				t.Errorf("Expected storage status '%s', got '%v'", tt.expectedStorage, storageComponent)
			}

			// Check timestamp is recent
			timeDiff := time.Since(response.Timestamp)
			if timeDiff > time.Second {
				t.Errorf("Health check timestamp seems old: %v", timeDiff)
			}
		})
	}
}

func TestHealthHandler_ResponseFormat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Use mock services to ensure predictable behavior
	mockStorage := storage.NewMockStorage()
	mockStorage.SetPingError(errors.New("storage unavailable"))

	mockLLM := services.NewMockLLMAPI()

	handler := NewHealthHandler(logger, mockStorage, mockLLM)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify all required fields are present
	if response.Status == "" {
		t.Error("Status field is empty")
	}

	if response.Service == "" {
		t.Error("Service field is empty")
	}

	if response.Timestamp.IsZero() {
		t.Error("Timestamp field is zero")
	}

	if response.Components == nil {
		t.Error("Components field is nil")
	}

	if len(response.Components) == 0 {
		t.Error("Components field is empty")
	}

	// Verify storage component is present
	if _, exists := response.Components["storage"]; !exists {
		t.Error("Storage component missing")
	}
}
