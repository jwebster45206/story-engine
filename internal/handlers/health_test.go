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

	"github.com/jwebster45206/roleplay-agent/internal/services"
)

func TestHealthHandler_ServeHTTP(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	tests := []struct {
		name           string
		setupCache     func() services.Cache
		expectedStatus int
		expectedHealth string
		expectedCache  string
	}{
		{
			name: "healthy cache",
			setupCache: func() services.Cache {
				mockCache := services.NewMockCache()
				mockCache.SetPingSuccess() // Cache is healthy
				return mockCache
			},
			expectedStatus: http.StatusOK,
			expectedHealth: "healthy",
			expectedCache:  "healthy",
		},
		{
			name: "unhealthy cache",
			setupCache: func() services.Cache {
				mockCache := services.NewMockCache()
				mockCache.SetPingError(errors.New("connection failed")) // Cache is unhealthy
				return mockCache
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedHealth: "degraded",
			expectedCache:  "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache()
			handler := NewHealthHandler(cache, logger)

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
			if response.Service != "roleplay-agent" {
				t.Errorf("Expected service 'roleplay-agent', got '%s'", response.Service)
			}

			// Check cache component status
			cacheStatus, exists := response.Components["cache"]
			if !exists {
				t.Error("Expected cache component in response")
			} else if cacheStatus != tt.expectedCache {
				t.Errorf("Expected cache status '%s', got '%s'", tt.expectedCache, cacheStatus)
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

	// Use a non-existent Redis to ensure predictable failure
	redisService := services.NewRedisService("localhost:9999", logger)
	handler := NewHealthHandler(redisService, logger)

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
}
