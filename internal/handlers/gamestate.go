package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type GameStateHandler struct {
	storage   services.Storage
	logger    *slog.Logger
	modelName string
}

func NewGameStateHandler(modelName string, storage services.Storage, logger *slog.Logger) *GameStateHandler {
	return &GameStateHandler{
		storage:   storage,
		logger:    logger,
		modelName: modelName,
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
	path := strings.TrimPrefix(r.URL.Path, "/v1/gamestate")
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
			if err := json.NewEncoder(w).Encode(response); err != nil {
				h.logger.Error("Failed to encode error response", "error", err)
			}
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
			if err := json.NewEncoder(w).Encode(response); err != nil {
				h.logger.Error("Failed to encode error response", "error", err)
			}
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
			if err := json.NewEncoder(w).Encode(response); err != nil {
				h.logger.Error("Failed to encode error response", "error", err)
			}
			return
		}
		h.handleDelete(w, r, gameStateID)

	default:
		h.logger.Warn("Method not allowed for game state endpoint", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		response := ErrorResponse{
			Error: "Method not allowed. Supported methods: POST, GET, DELETE",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
	}
}

func (h *GameStateHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Creating new game state")

	// Parse request body into GameState struct
	var gs state.GameState
	if err := json.NewDecoder(r.Body).Decode(&gs); err != nil {
		h.logger.Warn("Invalid JSON in request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Invalid JSON in request body",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	if gs.Validate() != nil {
		h.logger.Warn("Invalid game state data", "error", gs.Validate())
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Invalid game state data: " + gs.Validate().Error(),
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	// Get initial gamestate values from scenario
	s, err := h.storage.GetScenario(r.Context(), gs.Scenario)
	if err != nil {
		h.logger.Warn("Failed to load scenario", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Failed to load scenario: " + err.Error(),
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	// TODO: If using Anthropic provider, check that a valid content rating is set

	// Add the opening prompt to chat history
	if s.OpeningPrompt != "" {
		gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
			Role:    chat.ChatRoleAgent,
			Content: s.OpeningPrompt,
		})
	}

	// Initialize game state with scenario-level values
	gs.NPCs = s.NPCs
	gs.Location = s.OpeningLocation
	gs.WorldLocations = s.Locations
	gs.Inventory = s.OpeningInventory
	gs.Vars = s.Vars
	gs.ContingencyPrompts = s.ContingencyPrompts
	gs.ID = uuid.New()
	gs.TurnCounter = 0
	gs.SceneTurnCounter = 0
	gs.ModelName = h.modelName

	// If scenes are used, load the first scene
	if s.OpeningScene != "" {
		err = gs.LoadScene(s, s.OpeningScene)
		if err != nil {
			h.logger.Warn("Failed to load opening scene", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResponse{
				Error: "Failed to load opening scene: " + err.Error(),
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				h.logger.Error("Failed to encode error response", "error", err)
			}
			return
		}
	}

	if err := h.storage.SaveGameState(r.Context(), gs.ID, &gs); err != nil {
		h.logger.Error("Failed to save new game state", "error", err, "id", gs.ID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to create game state",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	h.logger.Debug("Game state created successfully", "id", gs.ID.String())
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(gs); err != nil {
		h.logger.Error("Failed to encode game state response", "error", err)
	}
}

func (h *GameStateHandler) handleRead(w http.ResponseWriter, r *http.Request, gameStateID uuid.UUID) {
	gs, err := h.storage.LoadGameState(r.Context(), gameStateID)
	if err != nil {
		h.logger.Error("Failed to load game state", "error", err, "id", gameStateID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to load game state",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	if gs == nil {
		h.logger.Warn("Game state not found", "id", gameStateID.String())
		w.WriteHeader(http.StatusNotFound)
		response := ErrorResponse{
			Error: "Game state not found",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(gs); err != nil {
		h.logger.Error("Failed to encode game state response", "error", err)
	}
}

func (h *GameStateHandler) handleDelete(w http.ResponseWriter, r *http.Request, gameStateID uuid.UUID) {
	if err := h.storage.DeleteGameState(r.Context(), gameStateID); err != nil {
		h.logger.Error("Failed to delete game state", "error", err, "id", gameStateID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to delete game state",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}
	h.logger.Debug("Game state deleted successfully", "id", gameStateID.String())
	w.WriteHeader(http.StatusNoContent)
}
