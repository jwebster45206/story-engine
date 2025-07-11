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
		setupLLM       func() services.LLMService
		expectedStatus int
		expectedHealth string
		expectedCache  string
		expectedOllama string
	}{
		{
			name: "all healthy",
			setupCache: func() services.Cache {
				mockCache := services.NewMockCache()
				mockCache.SetPingSuccess() // Cache is healthy
				return mockCache
			},
			setupLLM: func() services.LLMService {
				mockLLM := services.NewMockLLMAPI()
				return mockLLM
			},
			expectedStatus: http.StatusOK,
			expectedHealth: "healthy",
			expectedCache:  "healthy",
			expectedOllama: "healthy",
		},
		{
			name: "unhealthy cache",
			setupCache: func() services.Cache {
				mockCache := services.NewMockCache()
				mockCache.SetPingError(errors.New("connection failed")) // Cache is unhealthy
				return mockCache
			},
			setupLLM: func() services.LLMService {
				mockLLM := services.NewMockLLMAPI()
				return mockLLM
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedHealth: "degraded",
			expectedCache:  "unhealthy",
			expectedOllama: "healthy",
		},
		{
			name: "unhealthy ollama",
			setupCache: func() services.Cache {
				mockCache := services.NewMockCache()
				mockCache.SetPingSuccess()
				return mockCache
			},
			setupLLM: func() services.LLMService {
				mockLLM := services.NewMockLLMAPI()
				mockLLM.SetListModelsError(errors.New("ollama connection failed"))
				return mockLLM
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedHealth: "degraded",
			expectedCache:  "healthy",
			expectedOllama: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache()
			llmService := tt.setupLLM()
			handler := NewHealthHandler(cache, llmService, logger)

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
			cacheComponent, exists := response.Components["cache"]
			if !exists {
				t.Error("Expected cache component in response")
			} else if cacheComponent != tt.expectedCache {
				t.Errorf("Expected cache status '%s', got '%v'", tt.expectedCache, cacheComponent)
			}

			// Check ollama component status
			ollamaComponent, exists := response.Components["ollama"]
			if !exists {
				t.Error("Expected ollama component in response")
			} else {
				ollamaMap, ok := ollamaComponent.(map[string]interface{})
				if !ok {
					t.Errorf("Expected ollama component to be a map, got %T", ollamaComponent)
				} else {
					status, statusExists := ollamaMap["status"]
					if !statusExists {
						t.Error("Expected ollama status in component")
					} else if status != tt.expectedOllama {
						t.Errorf("Expected ollama status '%s', got '%v'", tt.expectedOllama, status)
					}

					if tt.expectedOllama == "healthy" {
						// Check that tags are present for healthy ollama
						if _, tagsExist := ollamaMap["tags"]; !tagsExist {
							t.Error("Expected tags in healthy ollama component")
						}
					}
				}
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
	mockCache := services.NewMockCache()
	mockCache.SetPingError(errors.New("cache unavailable"))

	mockLLM := services.NewMockLLMAPI()
	mockLLM.SetListModelsError(errors.New("ollama unavailable"))

	handler := NewHealthHandler(mockCache, mockLLM, logger)

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

	// Verify both cache and ollama components are present
	if _, exists := response.Components["cache"]; !exists {
		t.Error("Cache component missing")
	}

	if _, exists := response.Components["ollama"]; !exists {
		t.Error("Ollama component missing")
	}
}
