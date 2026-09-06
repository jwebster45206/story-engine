package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/httperror"
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
		httperror.Write(w, h.log, http.StatusInternalServerError, "Failed to list narrators")
		return
	}

	// Initialize as empty slice instead of nil
	narratorList := make([]map[string]any, 0)
	for _, narratorID := range narratorIDs {
		// Load each narrator to get details
		narrator, err := h.storage.GetNarrator(r.Context(), narratorID)
		if err != nil {
			h.log.Warn("Failed to load narrator", "error", err, "id", narratorID)
			continue
		}

		// Create a summary object with just the key fields
		narratorSummary := map[string]any{
			"id":          narrator.ID,
			"name":        narrator.Name,
			"description": narrator.Description,
		}
		narratorList = append(narratorList, narratorSummary)
	}

	data, err := json.Marshal(narratorList)
	if err != nil {
		h.log.Error("Failed to marshal narrator list", "error", err)
		httperror.Write(w, h.log, http.StatusInternalServerError, "Failed to process narrator list")
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
		httperror.Write(w, h.log, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *NarratorHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/narrators/")
	id := strings.TrimSpace(path)

	if id == "" || id == "/" {
		httperror.Write(w, h.log, http.StatusBadRequest, "Narrator ID is required in URL path (e.g., /v1/narrators/vincent_price)")
		return
	}

	// Security: prevent directory traversal
	if strings.Contains(id, "..") || strings.Contains(id, "/") {
		httperror.Write(w, h.log, http.StatusBadRequest, "Invalid narrator ID")
		return
	}

	// Load the narrator
	narrator, err := h.storage.GetNarrator(r.Context(), id)
	if err != nil {
		h.log.Error("Failed to load narrator", "error", err, "id", id)
		httperror.Write(w, h.log, http.StatusInternalServerError, "Failed to load narrator")
		return
	}

	if narrator == nil {
		httperror.Write(w, h.log, http.StatusNotFound, "Narrator not found")
		return
	}

	// Marshal the narrator
	data, err := json.Marshal(narrator)
	if err != nil {
		h.log.Error("Failed to marshal narrator", "error", err, "id", id)
		httperror.Write(w, h.log, http.StatusInternalServerError, "Failed to process narrator")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.log.Error("Failed to write response", "error", err, "id", id)
	}
}
