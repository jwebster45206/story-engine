package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *MonsterHandler) ListMonsters(w http.ResponseWriter, r *http.Request) {
	monsters, err := h.storage.ListMonsters(r.Context())
	if err != nil {
		h.logger.Error("Failed to list monsters", "error", err)
		http.Error(w, "Failed to list monsters", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"monsters": monsters,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *MonsterHandler) GetMonster(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/monsters/")
	templateID := strings.TrimSpace(path)

	if templateID == "" || templateID == "/" {
		http.Error(w, "Template ID is required in URL path (e.g., /v1/monsters/giant_rat)", http.StatusBadRequest)
		return
	}

	if strings.Contains(templateID, "..") || strings.Contains(templateID, "/") {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	monster, err := h.storage.GetMonster(r.Context(), templateID)
	if err != nil {
		h.logger.Error("Failed to get monster", "templateID", templateID, "error", err)
		http.Error(w, "Monster template not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(monster); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
