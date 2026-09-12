package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/auth"
	"github.com/jwebster45206/story-engine/internal/httperror"
	"github.com/jwebster45206/story-engine/internal/llm"
	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

// ProviderCatalog is the slice of the registry that HTTP handlers need.
type ProviderCatalog interface {
	Default() string
	Names() []string
	Info(name string) (llm.ProviderInfo, bool)
}

type GameStateHandler struct {
	storage storage.Storage
	logger  *slog.Logger
	catalog ProviderCatalog
}

func NewGameStateHandler(logger *slog.Logger, catalog ProviderCatalog, storage storage.Storage) *GameStateHandler {
	return &GameStateHandler{
		logger:  logger,
		catalog: catalog,
		storage: storage,
	}
}

func writeAuthorizeError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, auth.ErrUnauthorized):
		httperror.Write(w, logger, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, auth.ErrForbidden):
		httperror.Write(w, logger, http.StatusForbidden, "forbidden")
	case errors.Is(err, storage.ErrNotFound):
		httperror.Write(w, logger, http.StatusNotFound, "Game state not found")
	default:
		logger.Error("failed to authorize game", "error", err)
		httperror.Write(w, logger, http.StatusInternalServerError, "Failed to load game state")
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
			httperror.Write(w, h.logger, http.StatusBadRequest, "Invalid game state ID format")
			return
		}
	}

	switch r.Method {
	case http.MethodPost:
		h.handleCreate(w, r)

	case http.MethodGet:
		if gameStateID == uuid.Nil() {
			h.logger.Warn("GET request without game state ID")
			httperror.Write(w, h.logger, http.StatusBadRequest, "Game state ID is required for GET requests")
			return
		}
		h.handleRead(w, r, gameStateID)

	case http.MethodPatch:
		if gameStateID == uuid.Nil() {
			httperror.Write(w, h.logger, http.StatusBadRequest, "Game state ID is required for PATCH requests")
			return
		}
		h.handlePatch(w, r, gameStateID)

	case http.MethodDelete:
		if gameStateID == uuid.Nil() {
			h.logger.Warn("DELETE request without game state ID")
			httperror.Write(w, h.logger, http.StatusBadRequest, "Game state ID is required for DELETE requests")
			return
		}
		h.handleDelete(w, r, gameStateID)

	default:
		h.logger.Warn("Method not allowed for game state endpoint", "method", r.Method)
		httperror.Write(w, h.logger, http.StatusMethodNotAllowed, "Method not allowed. Supported methods: POST, GET, PATCH, DELETE")
	}
}

func (h *GameStateHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Creating new game state")

	p, err := auth.RequirePrincipal(r)
	if err != nil {
		httperror.Write(w, h.logger, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req state.GameState
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid JSON in request body", "error", err)
		httperror.Write(w, h.logger, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	if err := req.Validate(); err != nil {
		h.logger.Warn("Invalid create gamestate request", "error", err)
		httperror.Write(w, h.logger, http.StatusBadRequest, err.Error())
		return
	}
	req.Normalize()

	provider := req.Provider
	if provider == "" {
		provider = h.catalog.Default()
	}
	info, ok := h.catalog.Info(provider)
	if !ok {
		h.logger.Warn("Unknown provider on create", "provider", provider)
		httperror.Write(w, h.logger, http.StatusBadRequest, "Unknown provider: "+provider)
		return
	}

	// Get initial gamestate values from scenario
	s, err := h.storage.GetScenario(r.Context(), req.Scenario)
	if err != nil {
		h.logger.Warn("Failed to load scenario", "error", err)
		httperror.Write(w, h.logger, http.StatusBadRequest, "Failed to load scenario: "+err.Error())
		return
	}

	// Determine which narrator to use and load it ONCE (will be embedded in gamestate)
	var requestNarratorID string
	if req.Narrator != nil {
		requestNarratorID = req.Narrator.ID
	}
	narratorID := requestNarratorID
	if narratorID == "" {
		narratorID = s.NarratorID // Fall back to scenario's narrator
	}
	var narrator *scenario.Narrator
	if narratorID != "" {
		narrator, err = h.storage.GetNarrator(r.Context(), narratorID)
		if err != nil {
			h.logger.Warn("Failed to load narrator, continuing without narrator", "narrator_id", narratorID, "error", err)
		} else {
			h.logger.Debug("Loaded narrator for embedding", "narrator_id", narratorID, "name", narrator.Name, "source", map[bool]string{true: "request", false: "scenario"}[requestNarratorID != ""])
		}
	}

	// Create a new GameState; model_name is stamped from the provider config (client value ignored).
	gs := state.NewGameState(req.Scenario, narrator, provider, info.Model)
	gs.PrincipalID = p.ID
	gs.Rules = req.Rules
	gs.Temperature = req.Temperature

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
	var requestPCID string
	if req.PC != nil {
		requestPCID = req.PC.ID
	}
	pcID := requestPCID
	if pcID == "" {
		pcID = s.DefaultPC // Fall back to scenario's default PC
	}
	if pcID == "" {
		pcID = state.DefaultPCID
	}

	var loadedPC *character.PC
	pc, pcErr := h.storage.GetPC(r.Context(), pcID)
	if pcErr != nil {
		h.logger.Warn("Failed to load PC, trying fallback to classic", "pc_id", pcID, "error", pcErr)
		pc, pcErr = h.storage.GetPC(r.Context(), state.DefaultPCID)
		if pcErr != nil {
			h.logger.Error("Failed to load fallback PC 'classic'", "error", pcErr)
			// Continue without PC rather than failing - PC is optional for now
		}
	}
	if pc != nil {
		loadedPC = pc
		gs.PC = loadedPC
		h.logger.Debug("PC loaded successfully", "pc_id", loadedPC.ID, "name", loadedPC.Name, "source", map[bool]string{true: "request", false: "scenario"}[requestPCID != ""])
	}

	// Merge PC starting inventory with scenario starting inventory
	// GameState.Inventory is the canonical source - PC inventory is just a template
	inventoryMap := make(map[string]bool)

	// Add scenario inventory first
	for _, item := range s.OpeningInventory {
		inventoryMap[item] = true
	}

	// Add PC starting inventory (if PC loaded)
	if loadedPC != nil {
		for _, item := range loadedPC.Inventory {
			inventoryMap[item] = true
		}
	}

	// Convert map to slice (deduplicates automatically)
	gs.Inventory = make([]string, 0, len(inventoryMap))
	for item := range inventoryMap {
		gs.Inventory = append(gs.Inventory, item)
	}

	// Clear PC inventory to avoid confusion - gs.Inventory is now canonical
	if gs.PC != nil {
		gs.PC.Inventory = nil
	}

	// Add the opening prompt to chat history
	// If the scenario opening prompt contains %s and PC has an opening_prompt, inject it
	if s.OpeningPrompt != "" {
		openingPrompt := s.OpeningPrompt

		// Check if scenario prompt has placeholder and PC has opening prompt
		// Use gs.PC instead of loadedPC since that's the canonical reference
		if strings.Contains(openingPrompt, "%s") && gs.PC != nil && gs.PC.OpeningPrompt != "" {
			openingPrompt = fmt.Sprintf(openingPrompt, gs.PC.OpeningPrompt)
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
			httperror.Write(w, h.logger, http.StatusBadRequest, "Failed to load opening scene: "+err.Error())
			return
		}
	}

	// Resolve NPC templates: for any NPC in gs.NPCs that has a TemplateID set,
	// load the standalone NPC JSON from data/npcs/ and merge with the inline
	// definition (which may supply overrides like location or disposition).
	// This runs after LoadScene so that scene-level NPCs with TemplateIDs are also resolved.
	for npcKey, npc := range gs.NPCs {
		if npc.TemplateID == "" {
			continue // fully inline NPC, nothing to do
		}
		template, err := h.storage.GetNPC(r.Context(), npc.TemplateID)
		if err != nil {
			h.logger.Warn("Failed to load NPC template, keeping inline definition", "npc_key", npcKey, "template_id", npc.TemplateID, "error", err)
			continue
		}
		merged := character.NewNPCFromTemplate(template, &npc)
		if merged == nil {
			h.logger.Warn("Failed to merge NPC template, keeping inline definition", "npc_key", npcKey, "template_id", npc.TemplateID)
			continue
		}
		gs.NPCs[npcKey] = *merged
		h.logger.Debug("Loaded NPC from template", "npc_key", npcKey, "template_id", npc.TemplateID, "name", merged.Name)
	}

	// Load monster templates for any pre-placed monsters
	// Iterate through all locations and populate monsters that only have template_id set
	for locName, loc := range gs.WorldLocations {
		if loc.Monsters == nil {
			continue
		}
		for instanceID, monster := range loc.Monsters {
			// If monster has a template_id, load the full template
			if monster.TemplateID != "" {
				template, err := h.storage.GetMonster(r.Context(), monster.TemplateID)
				if err != nil {
					h.logger.Warn("Failed to load monster template, skipping", "instance_id", instanceID, "template_id", monster.TemplateID, "location", locName, "error", err)
					delete(loc.Monsters, instanceID)
					continue
				}

				monster.ID = instanceID
				monster.Location = locName

				// Spawn the monster using the template and scenario definition (which may contain overrides)
				spawnedMonster := gs.SpawnMonster(template, monster)
				if spawnedMonster == nil {
					h.logger.Warn("Failed to spawn monster", "instance_id", instanceID, "template_id", monster.TemplateID, "location", locName)
					delete(loc.Monsters, instanceID)
				} else {
					h.logger.Debug("Spawned monster from template", "instance_id", instanceID, "template_id", monster.TemplateID, "name", spawnedMonster.Name, "location", locName)
				}
			}
		}
		// Update the location in game state
		gs.WorldLocations[locName] = loc
	}

	if err := h.storage.CreateGameState(r.Context(), gs.ID, gs); err != nil {
		h.logger.Error("Failed to save new game state", "error", err, "id", gs.ID.String())
		httperror.Write(w, h.logger, http.StatusInternalServerError, "Failed to create game state")
		return
	}

	h.logger.Debug("Game state created successfully", "id", gs.ID.String())
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(gs); err != nil {
		h.logger.Error("Failed to encode game state response", "error", err)
	}
}

func (h *GameStateHandler) handleRead(w http.ResponseWriter, r *http.Request, gameStateID uuid.UUID) {
	if err := auth.AuthorizeGame(r, h.storage, gameStateID); err != nil {
		writeAuthorizeError(w, h.logger, err)
		return
	}
	gs, err := h.storage.LoadGameState(r.Context(), gameStateID)
	if err != nil {
		h.logger.Error("Failed to load game state", "error", err, "id", gameStateID.String())
		httperror.Write(w, h.logger, http.StatusInternalServerError, "Failed to load game state")
		return
	}
	if gs == nil {
		httperror.Write(w, h.logger, http.StatusNotFound, "Game state not found")
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
	if err := auth.AuthorizeGame(r, h.storage, gameStateID); err != nil {
		writeAuthorizeError(w, h.logger, err)
		return
	}
	existingGS, err := h.storage.LoadGameState(r.Context(), gameStateID)
	if err != nil {
		h.logger.Error("Failed to load game state", "error", err, "id", gameStateID.String())
		httperror.Write(w, h.logger, http.StatusInternalServerError, "Failed to load game state")
		return
	}
	if existingGS == nil {
		httperror.Write(w, h.logger, http.StatusNotFound, "Game state not found")
		return
	}

	// Parse patch request body
	var patchData state.GameState
	if err := json.NewDecoder(r.Body).Decode(&patchData); err != nil {
		h.logger.Warn("Invalid JSON in PATCH request body", "error", err)
		httperror.Write(w, h.logger, http.StatusBadRequest, "Invalid JSON in request body")
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
	if patchData.Rules != "" {
		updatedGS.Rules = patchData.Rules
	}
	if patchData.Temperature > 0 {
		updatedGS.Temperature = patchData.Temperature
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

	if err := h.storage.UpdateGameState(r.Context(), gameStateID, &updatedGS); err != nil {
		h.logger.Error("Failed to save patched game state", "error", err, "id", gameStateID.String())
		httperror.Write(w, h.logger, http.StatusInternalServerError, "Failed to save game state")
		return
	}

	h.logger.Info("Game state patched successfully", "id", gameStateID.String(), "scenario", updatedGS.Scenario, "location", updatedGS.Location)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedGS); err != nil {
		h.logger.Error("Failed to encode patched game state response", "error", err)
	}
}

func (h *GameStateHandler) handleDelete(w http.ResponseWriter, r *http.Request, gameStateID uuid.UUID) {
	if err := auth.AuthorizeGame(r, h.storage, gameStateID); err != nil {
		writeAuthorizeError(w, h.logger, err)
		return
	}
	if err := h.storage.DeleteGameState(r.Context(), gameStateID); err != nil {
		h.logger.Error("Failed to delete game state", "error", err, "id", gameStateID.String())
		httperror.Write(w, h.logger, http.StatusInternalServerError, "Failed to delete game state")
		return
	}
	h.logger.Debug("Game state deleted successfully", "id", gameStateID.String())
	w.WriteHeader(http.StatusNoContent)
}
