package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/httperror"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

type ScenarioHandler struct {
	log     *slog.Logger
	storage storage.Storage
}

// ListScenarios lists all available scenario files
func (h *ScenarioHandler) ListScenarios(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scenarios, err := h.storage.ListScenarios(ctx)
	if err != nil {
		h.log.Error("Failed to list scenarios", "error", err)
		httperror.Write(w, h.log, http.StatusInternalServerError, "Failed to list scenarios")
		return
	}
	data, err := json.Marshal(scenarios)
	if err != nil {
		h.log.Error("Failed to marshal scenario list", "error", err)
		httperror.Write(w, h.log, http.StatusInternalServerError, "Failed to process scenario list")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.log.Error("Failed to write scenario list response", "error", err)
	}
}

func NewScenarioHandler(log *slog.Logger, storage storage.Storage) *ScenarioHandler {
	return &ScenarioHandler{
		log:     log,
		storage: storage,
	}
}

func (h *ScenarioHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/v1/scenarios" || r.URL.Path == "/v1/scenarios/" {
			h.ListScenarios(w, r)
		} else {
			h.handleGet(w, r)
		}
	default:
		httperror.Write(w, h.log, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ScenarioHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/scenarios/")
	filename := strings.TrimSpace(path)

	if filename == "" || filename == "/scenarios" {
		httperror.Write(w, h.log, http.StatusBadRequest, "filename is required in URL path (e.g., /scenarios/pirate.json)")
		return
	}

	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		httperror.Write(w, h.log, http.StatusBadRequest, "Invalid filename")
		return
	}

	ctx := r.Context()
	scenario, err := h.storage.GetScenario(ctx, filename)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httperror.Write(w, h.log, http.StatusNotFound, "Scenario not found")
			return
		}
		h.log.Error("Failed to get scenario", "error", err, "filename", filename)
		httperror.Write(w, h.log, http.StatusInternalServerError, "Failed to retrieve scenario")
		return
	}

	data, err := json.Marshal(scenario)
	if err != nil {
		h.log.Error("Failed to marshal scenario", "error", err, "filename", filename)
		httperror.Write(w, h.log, http.StatusInternalServerError, "Failed to process scenario")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.log.Error("Failed to write response", "error", err, "filename", filename)
	}
}
