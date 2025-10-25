package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/storage"
)

// NarratorDataDir is the default path to the narrator data directory
const NarratorDataDir = "data/narrators"

type NarratorHandler struct {
	log     *slog.Logger
	storage storage.Storage
}

// ListNarrators lists all available narrator files
func (h *NarratorHandler) ListNarrators(w http.ResponseWriter, r *http.Request) {
	narratorIDs, err := h.storage.ListNarrators(r.Context())
	if err != nil {
		h.log.Error("Failed to list narrators", "error", err)
		http.Error(w, "Failed to list narrators", http.StatusInternalServerError)
		return
	}

	// Initialize as empty slice instead of nil
	narratorList := make([]map[string]interface{}, 0)
	for _, narratorID := range narratorIDs {
		// Load each narrator to get details
		narrator, err := h.storage.GetNarrator(r.Context(), narratorID)
		if err != nil {
			h.log.Warn("Failed to load narrator", "error", err, "id", narratorID)
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

func NewNarratorHandler(log *slog.Logger, storage storage.Storage) *NarratorHandler {
	return &NarratorHandler{
		log:     log,
		storage: storage,
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

	// Load the narrator
	narrator, err := h.storage.GetNarrator(r.Context(), id)
	if err != nil {
		h.log.Error("Failed to load narrator", "error", err, "id", id)
		http.Error(w, "Failed to load narrator", http.StatusInternalServerError)
		return
	}

	if narrator == nil {
		http.Error(w, "Narrator not found", http.StatusNotFound)
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
