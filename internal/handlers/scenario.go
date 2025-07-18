package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/services"
)

type ScenarioHandler struct {
	log     *slog.Logger
	storage services.Storage
}

func NewScenarioHandler(log *slog.Logger, storage services.Storage) *ScenarioHandler {
	return &ScenarioHandler{
		log:     log,
		storage: storage,
	}
}

func (h *ScenarioHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ScenarioHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/scenario/")
	filename := strings.TrimSpace(path)

	if filename == "" || filename == "/scenario" {
		http.Error(w, "filename is required in URL path (e.g., /scenario/pirate.json)", http.StatusBadRequest)
		return
	}

	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	scenario, err := h.storage.GetScenario(ctx, filename)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Scenario not found", http.StatusNotFound)
			return
		}
		h.log.Error("Failed to get scenario", "error", err, "filename", filename)
		http.Error(w, "Failed to retrieve scenario", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(scenario)
	if err != nil {
		h.log.Error("Failed to marshal scenario", "error", err, "filename", filename)
		http.Error(w, "Failed to process scenario", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
