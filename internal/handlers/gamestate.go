package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/internal/services"
	"github.com/jwebster45206/roleplay-agent/pkg/state"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type GameStateHandler struct {
	storage services.Storage
	logger  *slog.Logger
}

func NewGameStateHandler(storage services.Storage, logger *slog.Logger) *GameStateHandler {
	return &GameStateHandler{
		storage: storage,
		logger:  logger,
	}
}

// ServeHTTP handles HTTP requests for game state operations
// Routes:
// POST /gamestate        - Create new game state
// GET /gamestate/{id}    - Read game state by ID
// DELETE /gamestate/{id} - Delete game state by ID
func (h *GameStateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the path to extract ID for GET/DELETE operations
	path := strings.TrimPrefix(r.URL.Path, "/gamestate")
	var gameStateID uuid.UUID
	var err error

	if path != "" && path != "/" {
		// Extract ID from path like "/uuid" or "/{uuid}"
		idStr := strings.Trim(path, "/")
		gameStateID, err = uuid.Parse(idStr)
		if err != nil {
			h.logger.Warn("Invalid game state ID", "id", idStr, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResponse{
				Error: "Invalid game state ID format",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	switch r.Method {
	case http.MethodPost:
		h.handleCreate(w, r)
	case http.MethodGet:
		if gameStateID == uuid.Nil {
			h.logger.Warn("GET request without game state ID")
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResponse{
				Error: "Game state ID is required for GET requests",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		h.handleRead(w, r, gameStateID)
	case http.MethodDelete:
		if gameStateID == uuid.Nil {
			h.logger.Warn("DELETE request without game state ID")
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResponse{
				Error: "Game state ID is required for DELETE requests",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		h.handleDelete(w, r, gameStateID)
	default:
		h.logger.Warn("Method not allowed for game state endpoint", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		response := ErrorResponse{
			Error: "Method not allowed. Supported methods: POST, GET, DELETE",
		}
		json.NewEncoder(w).Encode(response)
	}
}

func (h *GameStateHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Creating new game state")

	gs := state.NewGameState()

	// Parse request body if provided (optional for create)
	// TODO: Not supported yet, but could be enabled later

	// Save the new game state
	if err := h.storage.SaveGameState(r.Context(), gs.ID, gs); err != nil {
		h.logger.Error("Failed to save new game state", "error", err, "id", gs.ID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to create game state",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	h.logger.Debug("Game state created successfully", "id", gs.ID.String())
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(gs)
}

func (h *GameStateHandler) handleRead(w http.ResponseWriter, r *http.Request, gameStateID uuid.UUID) {
	h.logger.Debug("Reading game state", "id", gameStateID.String())

	gs, err := h.storage.LoadGameState(r.Context(), gameStateID)
	if err != nil {
		h.logger.Error("Failed to load game state", "error", err, "id", gameStateID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to load game state",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if gs == nil {
		h.logger.Debug("Game state not found", "id", gameStateID.String())
		w.WriteHeader(http.StatusNotFound)
		response := ErrorResponse{
			Error: "Game state not found",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	h.logger.Debug("Game state loaded successfully", "id", gameStateID.String(), "chat_history_length", len(gs.ChatHistory))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(gs)
}

func (h *GameStateHandler) handleDelete(w http.ResponseWriter, r *http.Request, gameStateID uuid.UUID) {
	if err := h.storage.DeleteGameState(r.Context(), gameStateID); err != nil {
		h.logger.Error("Failed to delete game state", "error", err, "id", gameStateID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to delete game state",
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	h.logger.Debug("Game state deleted successfully", "id", gameStateID.String())
	w.WriteHeader(http.StatusNoContent)
}
