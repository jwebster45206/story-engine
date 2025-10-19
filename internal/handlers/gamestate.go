package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
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
// PATCH /gamestate/{id}  - Update game state
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

	case http.MethodPatch:
		if gameStateID == uuid.Nil {
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResponse{
				Error: "Game state ID is required for PATCH requests",
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				h.logger.Error("Failed to encode error response", "error", err)
			}
			return
		}
		h.handlePatch(w, r, gameStateID)

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
			Error: "Method not allowed. Supported methods: POST, GET, PATCH, DELETE",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
	}
}

func isCensoredModel(modelName string) bool {
	modelLower := strings.ToLower(modelName)
	if strings.Contains(modelLower, "gpt") ||
		strings.Contains(modelLower, "claude") ||
		strings.HasPrefix(modelLower, "text-davinci") ||
		strings.HasPrefix(modelLower, "text-curie") ||
		strings.HasPrefix(modelLower, "text-babbage") ||
		strings.HasPrefix(modelLower, "text-ada") ||
		strings.Contains(modelLower, "openai") ||
		strings.Contains(modelLower, "anthropic") {
		return true
	}
	return false
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

	// If using a censored model, check the scenario for compatibility
	if isCensoredModel(h.modelName) &&
		s.Rating != scenario.RatingG &&
		s.Rating != scenario.RatingPG &&
		s.Rating != scenario.RatingPG13 &&
		s.Rating != "PG13" {
		h.logger.Error("Attempt to use censored model with wrong scenario rating", "model", h.modelName, "rating", s.Rating)
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Censored model cannot be used with this scenario rating: " + s.Rating,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

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
	// Extract just the prompt strings from scenario-level contingency prompts
	gs.ContingencyPrompts = make([]string, 0, len(s.ContingencyPrompts))
	for _, cp := range s.ContingencyPrompts {
		gs.ContingencyPrompts = append(gs.ContingencyPrompts, cp.Prompt)
	}
	gs.ID = uuid.New()
	gs.TurnCounter = 0
	gs.SceneTurnCounter = 0
	gs.ModelName = h.modelName

	// Load PC from scenario (with fallback to classic)
	pcID := s.DefaultPC
	if pcID == "" {
		pcID = "classic"
	}
	pcPath := filepath.Join("data/pcs", pcID+".json")
	loadedPC, pcErr := actor.LoadPC(pcPath)
	if pcErr != nil {
		h.logger.Warn("Failed to load PC, trying fallback to classic", "pc_id", pcID, "error", pcErr)
		// Try fallback to classic
		loadedPC, pcErr = actor.LoadPC("data/pcs/classic.json")
		if pcErr != nil {
			h.logger.Error("Failed to load fallback PC 'classic'", "error", pcErr)
			// Continue without PC rather than failing - PC is optional for now
		}
	}
	if loadedPC != nil {
		gs.PC = loadedPC
		h.logger.Debug("PC loaded successfully", "pc_id", loadedPC.Spec.ID, "name", loadedPC.Spec.Name)
	}

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

// handlePatch updates an existing game state.
// It doesn't do extensive validation of the update, so use with caution.
// Integ tests are the current use case.
func (h *GameStateHandler) handlePatch(w http.ResponseWriter, r *http.Request, gameStateID uuid.UUID) {
	existingGS, err := h.storage.LoadGameState(r.Context(), gameStateID)
	if err != nil {
		h.logger.Error("Failed to load game state for patch", "error", err, "id", gameStateID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to load game state",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	if existingGS == nil {
		h.logger.Warn("Game state not found for patch", "id", gameStateID.String())
		w.WriteHeader(http.StatusNotFound)
		response := ErrorResponse{
			Error: "Game state not found",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	// Parse patch request body
	var patchData state.GameState
	if err := json.NewDecoder(r.Body).Decode(&patchData); err != nil {
		h.logger.Warn("Invalid JSON in PATCH request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Invalid JSON in request body",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	// Apply patch fields to existing gamestate (only non-zero values)
	updatedGS := *existingGS

	// Apply patch fields
	if patchData.SceneName != "" {
		updatedGS.SceneName = patchData.SceneName
	}
	if patchData.Location != "" {
		updatedGS.Location = patchData.Location
	}
	if patchData.TurnCounter != 0 {
		updatedGS.TurnCounter = patchData.TurnCounter
	}
	if patchData.SceneTurnCounter != 0 {
		updatedGS.SceneTurnCounter = patchData.SceneTurnCounter
	}
	if len(patchData.Inventory) > 0 {
		updatedGS.Inventory = patchData.Inventory
	}
	if len(patchData.ChatHistory) > 0 {
		updatedGS.ChatHistory = patchData.ChatHistory
	}
	if len(patchData.Vars) > 0 {
		updatedGS.Vars = patchData.Vars
	}
	if len(patchData.NPCs) > 0 {
		updatedGS.NPCs = patchData.NPCs
	}
	if len(patchData.WorldLocations) > 0 {
		updatedGS.WorldLocations = patchData.WorldLocations
	}
	if len(patchData.ContingencyPrompts) > 0 {
		updatedGS.ContingencyPrompts = patchData.ContingencyPrompts
	}
	if patchData.IsEnded != existingGS.IsEnded {
		updatedGS.IsEnded = patchData.IsEnded
	}

	if err := h.storage.SaveGameState(r.Context(), gameStateID, &updatedGS); err != nil {
		h.logger.Error("Failed to save patched game state", "error", err, "id", gameStateID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to save game state",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	h.logger.Info("Game state patched successfully", "id", gameStateID.String(), "scenario", updatedGS.Scenario, "location", updatedGS.Location)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedGS); err != nil {
		h.logger.Error("Failed to encode patched game state response", "error", err)
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
