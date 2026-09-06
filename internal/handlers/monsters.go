package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/httperror"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

type MonsterHandler struct {
	logger  *slog.Logger
	storage storage.Storage
}

func NewMonsterHandler(logger *slog.Logger, storage storage.Storage) *MonsterHandler {
	return &MonsterHandler{
		logger:  logger,
		storage: storage,
	}
}

func (h *MonsterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/v1/monsters" || r.URL.Path == "/v1/monsters/" {
			h.ListMonsters(w, r)
		} else {
			h.GetMonster(w, r)
		}
	default:
		httperror.Write(w, h.logger, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *MonsterHandler) ListMonsters(w http.ResponseWriter, r *http.Request) {
	monsters, err := h.storage.ListMonsters(r.Context())
	if err != nil {
		h.logger.Error("Failed to list monsters", "error", err)
		httperror.Write(w, h.logger, http.StatusInternalServerError, "Failed to list monsters")
		return
	}

	response := map[string]any{
		"monsters": monsters,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
		httperror.Write(w, h.logger, http.StatusInternalServerError, "Failed to encode response")
		return
	}
}

func (h *MonsterHandler) GetMonster(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/monsters/")
	templateID := strings.TrimSpace(path)

	if templateID == "" || templateID == "/" {
		httperror.Write(w, h.logger, http.StatusBadRequest, "Template ID is required in URL path (e.g., /v1/monsters/giant_rat)")
		return
	}

	if strings.Contains(templateID, "..") || strings.Contains(templateID, "/") {
		httperror.Write(w, h.logger, http.StatusBadRequest, "Invalid template ID")
		return
	}

	monster, err := h.storage.GetMonster(r.Context(), templateID)
	if err != nil {
		h.logger.Error("Failed to get monster", "templateID", templateID, "error", err)
		httperror.Write(w, h.logger, http.StatusNotFound, "Monster template not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(monster); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
		httperror.Write(w, h.logger, http.StatusInternalServerError, "Failed to encode response")
		return
	}
}
