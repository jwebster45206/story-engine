package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/storage"
	"github.com/jwebster45206/story-engine/pkg/actor"
)

type PCHandler struct {
	log     *slog.Logger
	storage storage.Storage
}

// ListPCs lists all available PC files
func (h *PCHandler) ListPCs(w http.ResponseWriter, r *http.Request) {
	pcIDs, err := h.storage.ListPCs(r.Context())
	if err != nil {
		h.log.Error("Failed to list PCs", "error", err)
		http.Error(w, "Failed to list PCs", http.StatusInternalServerError)
		return
	}

	// Initialize as empty slice instead of nil
	pcList := make([]map[string]interface{}, 0)
	for _, pcID := range pcIDs {
		// Load each PC spec to get details
		spec, err := h.storage.GetPCSpec(r.Context(), pcID)
		if err != nil {
			h.log.Warn("Failed to load PC spec", "error", err, "id", pcID)
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

func NewPCHandler(log *slog.Logger, storage storage.Storage) *PCHandler {
	return &PCHandler{
		log:     log,
		storage: storage,
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

	// Load the PC spec by ID (storage handles path construction)
	pcSpec, err := h.storage.GetPCSpec(r.Context(), id)
	if err != nil {
		if err.Error() == "PC spec not found" {
			http.Error(w, "PC not found", http.StatusNotFound)
			return
		}
		h.log.Error("Failed to load PC spec", "error", err, "id", id)
		http.Error(w, "Failed to load PC", http.StatusInternalServerError)
		return
	}

	// Build the PC from the spec
	loadedPC, err := actor.NewPCFromSpec(pcSpec)
	if err != nil {
		h.log.Error("Failed to build PC from spec", "error", err, "id", id)
		http.Error(w, "Failed to build PC", http.StatusInternalServerError)
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
