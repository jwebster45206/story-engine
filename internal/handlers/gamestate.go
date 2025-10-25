package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type GameStateHandler struct {
	storage   storage.Storage
	logger    *slog.Logger
	modelName string
}

func NewGameStateHandler(logger *slog.Logger, modelName string, storage storage.Storage) *GameStateHandler {
	return &GameStateHandler{
		logger:    logger,
		modelName: modelName,
		storage:   storage,
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

// CreateGameStateRequest defines the request body for creating a new game state
type CreateGameStateRequest struct {
	Scenario   string `json:"scenario"`              // Required: scenario filename
	NarratorID string `json:"narrator_id,omitempty"` // Optional: override scenario's narrator
	PCID       string `json:"pc_id,omitempty"`       // Optional: override scenario's default PC
}

// normalizeID converts a string to lowercase snake_case for consistent IDs.
// It handles spaces, hyphens, dots, and camelCase/PascalCase.
func normalizeID(s string) string {
	if s == "" {
		return ""
	}

	var out strings.Builder
	prevUnderscore := false
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			r = r + ('a' - 'A')
		}
		switch {
		case r == '.':
			out.WriteRune('.')
			prevUnderscore = false

		case r == ' ' || r == '-' || r == '_':
			if !prevUnderscore && i > 0 {
				out.WriteRune('_')
				prevUnderscore = true
			}

		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			out.WriteRune(r)
			prevUnderscore = false

		default:
			// Ignore other characters
		}
	}
	return out.String()
}

// ensureJSONExtension adds .json extension if not present
func ensureJSONExtension(s string) string {
	if s == "" {
		return ""
	}
	if !strings.HasSuffix(s, ".json") {
		return s + ".json"
	}
	return s
}

// stripJSONExtension removes .json extension if present
func stripJSONExtension(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimSuffix(s, ".json")
}

// Normalize normalizes all ID fields to lowercase snake_case,
// ensures .json extension for scenario, and strips .json from narrator/pc IDs
func (req *CreateGameStateRequest) Normalize() {
	req.Scenario = normalizeID(req.Scenario)
	req.Scenario = ensureJSONExtension(req.Scenario)

	req.NarratorID = normalizeID(req.NarratorID)
	req.NarratorID = stripJSONExtension(req.NarratorID)

	req.PCID = normalizeID(req.PCID)
	req.PCID = stripJSONExtension(req.PCID)
}

func (h *GameStateHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Creating new game state")

	// Parse request body into CreateGameStateRequest struct
	var req CreateGameStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	// Normalize all input fields to snake_case
	req.Normalize()

	// Validate required fields
	if req.Scenario == "" {
		h.logger.Warn("Missing required field: scenario")
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "scenario field is required",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	// Get initial gamestate values from scenario
	s, err := h.storage.GetScenario(r.Context(), req.Scenario)
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

	// Determine which narrator to use and load it ONCE (will be embedded in gamestate)
	narratorID := req.NarratorID // Use request override if provided
	if narratorID == "" {
		narratorID = s.NarratorID // Fall back to scenario's narrator
	}
	var narrator *scenario.Narrator
	if narratorID != "" {
		narrator, err = h.storage.GetNarrator(r.Context(), narratorID)
		if err != nil {
			h.logger.Warn("Failed to load narrator, continuing without narrator", "narrator_id", narratorID, "error", err)
		} else {
			h.logger.Debug("Loaded narrator for embedding", "narrator_id", narratorID, "name", narrator.Name, "source", map[bool]string{true: "request", false: "scenario"}[req.NarratorID != ""])
		}
	}

	// Create a new GameState with embedded narrator
	gs := state.NewGameState(req.Scenario, narrator, h.modelName)

	// Initialize game state with scenario-level values
	gs.NPCs = s.NPCs
	gs.Location = s.OpeningLocation
	gs.WorldLocations = s.Locations
	gs.Vars = s.Vars
	// ContingencyPrompts field is for runtime-added custom prompts only
	// Scenario-level prompts are already filtered and added in GetContingencyPrompts()
	// so we don't copy them here to avoid duplication
	gs.ContingencyPrompts = make([]string, 0)

	// Determine which PC to use
	pcID := req.PCID // Use request override if provided
	if pcID == "" {
		pcID = s.DefaultPC // Fall back to scenario's default PC
	}
	if pcID == "" {
		pcID = "classic" // Final fallback to classic
	}

	var loadedPC *actor.PC
	pcPath := filepath.Join("data/pcs", pcID+".json")
	pcSpec, pcErr := h.storage.GetPCSpec(r.Context(), pcPath)
	if pcErr != nil {
		h.logger.Warn("Failed to load PC spec, trying fallback to classic", "pc_id", pcID, "error", pcErr)
		// Try fallback to classic
		pcSpec, pcErr = h.storage.GetPCSpec(r.Context(), "data/pcs/classic.json")
		if pcErr != nil {
			h.logger.Error("Failed to load fallback PC 'classic'", "error", pcErr)
			// Continue without PC rather than failing - PC is optional for now
		}
	}
	if pcSpec != nil {
		var err error
		loadedPC, err = actor.NewPCFromSpec(pcSpec)
		if err != nil {
			h.logger.Error("Failed to construct PC from spec", "pc_id", pcSpec.ID, "error", err)
		} else {
			gs.PC = loadedPC
			h.logger.Debug("PC loaded successfully", "pc_id", loadedPC.Spec.ID, "name", loadedPC.Spec.Name, "source", map[bool]string{true: "request", false: "scenario"}[req.PCID != ""])
		}
	}

	// Merge PC starting inventory with scenario starting inventory
	// GameState.Inventory is the canonical source - PC inventory is just a template
	inventoryMap := make(map[string]bool)

	// Add scenario inventory first
	for _, item := range s.OpeningInventory {
		inventoryMap[item] = true
	}

	// Add PC starting inventory (if PC loaded)
	if loadedPC != nil && loadedPC.Spec != nil && loadedPC.Spec.Inventory != nil {
		for _, item := range loadedPC.Spec.Inventory {
			inventoryMap[item] = true
		}
	}

	// Convert map to slice (deduplicates automatically)
	gs.Inventory = make([]string, 0, len(inventoryMap))
	for item := range inventoryMap {
		gs.Inventory = append(gs.Inventory, item)
	}

	// Clear PC inventory to avoid confusion - gs.Inventory is now canonical
	// PC.Spec.Inventory was just a template for starting items
	if gs.PC != nil && gs.PC.Spec != nil {
		gs.PC.Spec.Inventory = nil
	}

	// Add the opening prompt to chat history
	// If the scenario opening prompt contains %s and PC has an opening_prompt, inject it
	if s.OpeningPrompt != "" {
		openingPrompt := s.OpeningPrompt

		// Check if scenario prompt has placeholder and PC has opening prompt
		if strings.Contains(openingPrompt, "%s") && loadedPC != nil && loadedPC.Spec.OpeningPrompt != "" {
			openingPrompt = fmt.Sprintf(openingPrompt, loadedPC.Spec.OpeningPrompt)
		}

		gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
			Role:    chat.ChatRoleAgent,
			Content: openingPrompt,
		})
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

	if err := h.storage.SaveGameState(r.Context(), gs.ID, gs); err != nil {
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
