package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// NarratorDataDir is the default path to the narrator data directory
const NarratorDataDir = "data/narrators"

type NarratorHandler struct {
	log     *slog.Logger
	dataDir string
}

// ListNarrators lists all available narrator files
func (h *NarratorHandler) ListNarrators(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.dataDir)
	if err != nil {
		h.log.Error("Failed to read narrators directory", "error", err, "dir", h.dataDir)
		http.Error(w, "Failed to list narrators", http.StatusInternalServerError)
		return
	}

	// Initialize as empty slice instead of nil
	narratorList := make([]map[string]interface{}, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Read JSON directly
		narratorPath := filepath.Join(h.dataDir, entry.Name())
		jsonData, err := os.ReadFile(narratorPath)
		if err != nil {
			h.log.Warn("Failed to read narrator file", "error", err, "file", entry.Name())
			continue
		}

		// Parse the narrator
		var narrator scenario.Narrator
		if err := json.Unmarshal(jsonData, &narrator); err != nil {
			h.log.Warn("Failed to parse narrator file", "error", err, "file", entry.Name())
			continue
		}

		// Create a summary object with just the key fields
		narratorSummary := map[string]interface{}{
			"id":          narrator.ID,
			"name":        narrator.Name,
			"description": narrator.Description,
		}
		narratorList = append(narratorList, narratorSummary)
	}

	data, err := json.Marshal(narratorList)
	if err != nil {
		h.log.Error("Failed to marshal narrator list", "error", err)
		http.Error(w, "Failed to process narrator list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.log.Error("Failed to write narrator list response", "error", err)
	}
}

func NewNarratorHandler(log *slog.Logger, dataDir string) *NarratorHandler {
	return &NarratorHandler{
		log:     log,
		dataDir: dataDir,
	}
}

func (h *NarratorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/v1/narrators" || r.URL.Path == "/v1/narrators/" {
			h.ListNarrators(w, r)
		} else {
			h.handleGet(w, r)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *NarratorHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/narrators/")
	id := strings.TrimSpace(path)

	if id == "" || id == "/" {
		http.Error(w, "Narrator ID is required in URL path (e.g., /v1/narrators/vincent_price)", http.StatusBadRequest)
		return
	}

	// Security: prevent directory traversal
	if strings.Contains(id, "..") || strings.Contains(id, "/") {
		http.Error(w, "Invalid narrator ID", http.StatusBadRequest)
		return
	}

	// Construct the file path
	filename := id + ".json"
	narratorPath := filepath.Join(h.dataDir, filename)

	// Check if file exists
	if _, err := os.Stat(narratorPath); os.IsNotExist(err) {
		http.Error(w, "Narrator not found", http.StatusNotFound)
		return
	}

	// Read and parse the narrator file directly
	jsonData, err := os.ReadFile(narratorPath)
	if err != nil {
		h.log.Error("Failed to read narrator file", "error", err, "id", id)
		http.Error(w, "Failed to load narrator", http.StatusInternalServerError)
		return
	}

	var narrator scenario.Narrator
	if err := json.Unmarshal(jsonData, &narrator); err != nil {
		h.log.Error("Failed to parse narrator file", "error", err, "id", id)
		http.Error(w, "Failed to parse narrator", http.StatusInternalServerError)
		return
	}

	// Validate narrator ID matches filename
	if narrator.ID != id {
		h.log.Error("Narrator ID mismatch", "file_id", id, "json_id", narrator.ID)
		http.Error(w, "Narrator ID mismatch", http.StatusInternalServerError)
		return
	}

	// Marshal the narrator
	data, err := json.Marshal(narrator)
	if err != nil {
		h.log.Error("Failed to marshal narrator", "error", err, "id", id)
		http.Error(w, "Failed to process narrator", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.log.Error("Failed to write response", "error", err, "id", id)
	}
}
