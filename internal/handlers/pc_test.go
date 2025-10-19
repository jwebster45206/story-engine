package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// testPCDataDir is the relative path from the test location to the PC data
const testPCDataDir = "../../data/pcs"

func TestPCHandler_ListPCs(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewPCHandler(log, testPCDataDir)

	req := httptest.NewRequest(http.MethodGet, "/v1/pcs", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListPCs() status = %d, want %d", w.Code, http.StatusOK)
	}

	var pcList []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &pcList); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should have at least the real PCs we created (pirate_captain, classic, alexandra_kane)
	if len(pcList) < 3 {
		t.Errorf("ListPCs() returned %d PCs, want at least 3", len(pcList))
	}

	// Check that the summary fields are present
	for _, pc := range pcList {
		if _, ok := pc["id"]; !ok {
			t.Error("PC summary missing 'id' field")
		}
		if _, ok := pc["name"]; !ok {
			t.Error("PC summary missing 'name' field")
		}
		if _, ok := pc["class"]; !ok {
			t.Error("PC summary missing 'class' field")
		}
		if _, ok := pc["level"]; !ok {
			t.Error("PC summary missing 'level' field")
		}
		if _, ok := pc["race"]; !ok {
			t.Error("PC summary missing 'race' field")
		}
		if _, ok := pc["pronouns"]; !ok {
			t.Error("PC summary missing 'pronouns' field")
		}
	}
}

func TestPCHandler_GetPC(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewPCHandler(log, testPCDataDir)

	// Test with pirate_captain which we know exists
	req := httptest.NewRequest(http.MethodGet, "/v1/pcs/pirate_captain", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetPC() status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var pc map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &pc); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check key fields
	if pc["id"] != "pirate_captain" {
		t.Errorf("GetPC() id = %v, want 'pirate_captain'", pc["id"])
	}
	if pc["name"] != "Captain Jack Sparrow" {
		t.Errorf("GetPC() name = %v, want 'Captain Jack Sparrow'", pc["name"])
	}
	if pc["class"] != "rogue" {
		t.Errorf("GetPC() class = %v, want 'rogue'", pc["class"])
	}

	// Check that stats are present
	if _, ok := pc["stats"]; !ok {
		t.Error("GetPC() response missing 'stats' field")
	}
}

func TestPCHandler_GetPC_Classic(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewPCHandler(log, testPCDataDir)

	// Test with classic which we know exists
	req := httptest.NewRequest(http.MethodGet, "/v1/pcs/classic", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetPC() status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var pc map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &pc); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check key fields
	if pc["id"] != "classic" {
		t.Errorf("GetPC() id = %v, want 'classic'", pc["id"])
	}
	if pc["name"] != "Adventurer" {
		t.Errorf("GetPC() name = %v, want 'Adventurer'", pc["name"])
	}
}

func TestPCHandler_GetPC_NotFound(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewPCHandler(log, testPCDataDir)

	req := httptest.NewRequest(http.MethodGet, "/v1/pcs/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GetPC() with nonexistent PC status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestPCHandler_GetPC_InvalidID(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewPCHandler(log, testPCDataDir)

	testCases := []struct {
		name string
		path string
	}{
		{"directory traversal", "/v1/pcs/../../../etc/passwd"},
		{"path with slash", "/v1/pcs/subdir/pc"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: status = %d, want %d", tc.name, w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestPCHandler_MethodNotAllowed(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewPCHandler(log, testPCDataDir)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/v1/pcs", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s method status = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestPCHandler_ListPCs_WithTrailingSlash(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewPCHandler(log, testPCDataDir)

	req := httptest.NewRequest(http.MethodGet, "/v1/pcs/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListPCs() with trailing slash status = %d, want %d", w.Code, http.StatusOK)
	}
}
