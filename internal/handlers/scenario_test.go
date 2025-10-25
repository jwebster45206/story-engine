package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func TestScenarioHandler_ServeHTTP(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	mockSt := storage.NewMockStorage()
	mockSt.AddScenario("pirate.json", &scenario.Scenario{
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
		},
		NPCs: map[string]actor.NPC{
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
	})

	handler := NewScenarioHandler(logger, mockSt)

	req := httptest.NewRequest("GET", "/v1/scenarios/pirate.json", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response scenario.Scenario
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if !strings.Contains(response.Name, "Pirate") {
		t.Errorf("Expected scenario name to contain 'Pirate', got %q", response.Name)
	}
}
