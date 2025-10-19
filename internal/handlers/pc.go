package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/pc"
)

// PCDataDir is the default path to the PC data directory
const PCDataDir = "data/pcs"

type PCHandler struct {
	log     *slog.Logger
	dataDir string
}

// ListPCs lists all available PC files
func (h *PCHandler) ListPCs(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.dataDir)
	if err != nil {
		h.log.Error("Failed to read PCs directory", "error", err, "dir", h.dataDir)
		http.Error(w, "Failed to list PCs", http.StatusInternalServerError)
		return
	}

	// Initialize as empty slice instead of nil
	pcList := make([]map[string]interface{}, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Read JSON directly without building the full PC/Actor
		pcPath := filepath.Join(h.dataDir, entry.Name())
		jsonData, err := os.ReadFile(pcPath)
		if err != nil {
			h.log.Warn("Failed to read PC file", "error", err, "file", entry.Name())
			continue
		}

		// Parse just the PCSpec (no Actor building)
		var spec pc.PCSpec
		if err := json.Unmarshal(jsonData, &spec); err != nil {
			h.log.Warn("Failed to parse PC file", "error", err, "file", entry.Name())
			continue
		}

		// Create a summary object with just the key fields
		pcSummary := map[string]interface{}{
			"id":       spec.ID,
			"name":     spec.Name,
			"class":    spec.Class,
			"level":    spec.Level,
			"race":     spec.Race,
			"pronouns": spec.Pronouns,
		}
		pcList = append(pcList, pcSummary)
	}

	data, err := json.Marshal(pcList)
	if err != nil {
		h.log.Error("Failed to marshal PC list", "error", err)
		http.Error(w, "Failed to process PC list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.log.Error("Failed to write PC list response", "error", err)
	}
}

func NewPCHandler(log *slog.Logger, dataDir string) *PCHandler {
	return &PCHandler{
		log:     log,
		dataDir: dataDir,
	}
}

func (h *PCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/v1/pcs" || r.URL.Path == "/v1/pcs/" {
			h.ListPCs(w, r)
		} else {
			h.handleGet(w, r)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PCHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/pcs/")
	id := strings.TrimSpace(path)

	if id == "" || id == "/" {
		http.Error(w, "PC ID is required in URL path (e.g., /v1/pcs/pirate_captain)", http.StatusBadRequest)
		return
	}

	// Security: prevent directory traversal
	if strings.Contains(id, "..") || strings.Contains(id, "/") {
		http.Error(w, "Invalid PC ID", http.StatusBadRequest)
		return
	}

	// Construct the file path
	filename := id + ".json"
	pcPath := filepath.Join(h.dataDir, filename)

	// Check if file exists
	if _, err := os.Stat(pcPath); os.IsNotExist(err) {
		http.Error(w, "PC not found", http.StatusNotFound)
		return
	}

	// Load the PC
	loadedPC, err := pc.LoadPC(pcPath)
	if err != nil {
		h.log.Error("Failed to load PC", "error", err, "id", id)
		http.Error(w, "Failed to load PC", http.StatusInternalServerError)
		return
	}

	// Marshal the PC (uses custom MarshalJSON that reads from Actor)
	data, err := json.Marshal(loadedPC)
	if err != nil {
		h.log.Error("Failed to marshal PC", "error", err, "id", id)
		http.Error(w, "Failed to process PC", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.log.Error("Failed to write response", "error", err, "id", id)
	}
}
