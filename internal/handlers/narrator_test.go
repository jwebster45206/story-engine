package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNarratorHandler_ListNarrators(t *testing.T) {
	// Create a temp directory with test narrators
	tempDir := t.TempDir()

	// Create test narrator files
	narrator1 := `{
		"id": "classic",
		"name": "Classic Narrator",
		"description": "Traditional storytelling voice",
		"prompts": ["Be clear and direct"]
	}`

	narrator2 := `{
		"id": "spooky",
		"name": "Spooky Narrator",
		"description": "Dark and mysterious",
		"prompts": ["Use dramatic language"]
	}`

	if err := os.WriteFile(filepath.Join(tempDir, "classic.json"), []byte(narrator1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "spooky.json"), []byte(narrator2), 0644); err != nil {
		t.Fatal(err)
	}

	// Create handler
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewNarratorHandler(log, tempDir)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/narrators", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var narratorList []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &narratorList); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(narratorList) != 2 {
		t.Errorf("Expected 2 narrators, got %d", len(narratorList))
	}

	// Verify first narrator
	if narratorList[0]["id"] != "classic" {
		t.Errorf("Expected id 'classic', got %v", narratorList[0]["id"])
	}
	if narratorList[0]["name"] != "Classic Narrator" {
		t.Errorf("Expected name 'Classic Narrator', got %v", narratorList[0]["name"])
	}
}

func TestNarratorHandler_GetNarrator(t *testing.T) {
	// Create a temp directory with a test narrator
	tempDir := t.TempDir()

	narratorJSON := `{
		"id": "vincent_price",
		"name": "Vincent Price",
		"description": "Gothic horror style",
		"prompts": [
			"Use dramatic language",
			"Add suspense and dread"
		]
	}`

	if err := os.WriteFile(filepath.Join(tempDir, "vincent_price.json"), []byte(narratorJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create handler
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewNarratorHandler(log, tempDir)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/narrators/vincent_price", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var narrator map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &narrator); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if narrator["id"] != "vincent_price" {
		t.Errorf("Expected id 'vincent_price', got %v", narrator["id"])
	}
	if narrator["name"] != "Vincent Price" {
		t.Errorf("Expected name 'Vincent Price', got %v", narrator["name"])
	}

	// Verify prompts
	prompts, ok := narrator["prompts"].([]interface{})
	if !ok {
		t.Fatal("Expected prompts to be an array")
	}
	if len(prompts) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(prompts))
	}
}

func TestNarratorHandler_GetNarrator_NotFound(t *testing.T) {
	tempDir := t.TempDir()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewNarratorHandler(log, tempDir)

	req := httptest.NewRequest(http.MethodGet, "/v1/narrators/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestNarratorHandler_GetNarrator_InvalidID(t *testing.T) {
	tempDir := t.TempDir()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewNarratorHandler(log, tempDir)

	tests := []struct {
		name string
		path string
	}{
		{"directory traversal", "/v1/narrators/../secret"},
		{"path with slash", "/v1/narrators/path/with/slash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for %s, got %d", tt.name, w.Code)
			}
		})
	}
}

func TestNarratorHandler_MethodNotAllowed(t *testing.T) {
	tempDir := t.TempDir()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewNarratorHandler(log, tempDir)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/v1/narrators", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s, got %d", method, w.Code)
			}
		})
	}
}
