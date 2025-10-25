package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func TestNarratorHandler_ListNarrators(t *testing.T) {
	// Create mock storage
	mockStorage := storage.NewMockStorage()

	// Add test narrators
	mockStorage.AddNarrator("classic", &scenario.Narrator{
		ID:          "classic",
		Name:        "Classic Narrator",
		Description: "Traditional storytelling voice",
		Prompts:     []string{"Be clear and direct"},
	})

	mockStorage.AddNarrator("spooky", &scenario.Narrator{
		ID:          "spooky",
		Name:        "Spooky Narrator",
		Description: "Dark and mysterious",
		Prompts:     []string{"Use dramatic language"},
	})

	// Create handler
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewNarratorHandler(log, mockStorage)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/narrators", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %v", w.Code)
	}

	var narrators []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &narrators); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(narrators) != 2 {
		t.Fatalf("expected 2 narrators, got %d", len(narrators))
	}
}

func TestNarratorHandler_GetNarrator(t *testing.T) {
	// Create mock storage
	mockStorage := storage.NewMockStorage()

	// Add test narrator
	mockStorage.AddNarrator("classic", &scenario.Narrator{
		ID:          "classic",
		Name:        "Classic Narrator",
		Description: "Traditional storytelling voice",
		Prompts:     []string{"Be clear and direct"},
	})

	// Create handler
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewNarratorHandler(log, mockStorage)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/narrators/classic", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %v", w.Code)
	}

	var narrator scenario.Narrator
	if err := json.Unmarshal(w.Body.Bytes(), &narrator); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if narrator.ID != "classic" {
		t.Fatalf("expected narrator ID 'classic', got %s", narrator.ID)
	}
}
