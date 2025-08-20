package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// ChatHandler handles chat requests
type ChatHandler struct {
	llmService services.LLMService
	logger     *slog.Logger
	storage    services.Storage

	// For background meta update cancellation
	metaCancelMu sync.Mutex
	metaCancel   map[uuid.UUID]context.CancelFunc
}

// NewChatHandler creates a new chat handler
func NewChatHandler(llmService services.LLMService, logger *slog.Logger, storage services.Storage) *ChatHandler {
	return &ChatHandler{
		llmService: llmService,
		logger:     logger,
		storage:    storage,
		metaCancel: make(map[uuid.UUID]context.CancelFunc),
	}
}

const PromptHistoryLimit = 6

// ServeHTTP handles HTTP requests for chat.
// This is the primary endpoint for user interaction with the LLM.
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow POST method and check for /v1/chat path
	if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/v1/chat") {
		h.logger.Warn("Method not allowed for chat endpoint",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr)

		w.WriteHeader(http.StatusMethodNotAllowed)
		response := ErrorResponse{
			Error: "Method not allowed. Only POST is supported at /v1/chat.",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding chat error response",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path)
		}
		return
	}

	h.logger.Debug("Chat endpoint accessed",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr)

	var request chat.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Warn("Invalid request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Invalid request body. Expected JSON with 'message' field.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Validate request
	if err := request.Validate(); err != nil {
		h.logger.Warn("Invalid chat request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Load game state
	gs, err := h.storage.LoadGameState(r.Context(), request.GameStateID)
	if err != nil {
		h.logger.Error("Error loading game state", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to load game state.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	if gs == nil {
		h.logger.Warn("Game state not found", "requested_id", request.GameStateID.String())
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Game state not found. Please provide a valid game state ID.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Get Scenario for the chat
	scenario, err := h.storage.GetScenario(r.Context(), gs.Scenario)
	if err != nil {
		h.logger.Error("Error loading scenario for chat", "error", err, "scenario_filename", gs.Scenario)
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to load scenario for chat.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	cmdResult, err := gs.TryHandleCommand(request.Message)
	if err != nil {
		h.logger.Error("Error handling command in chat", "error", err, "command", request.Message)
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to handle command in chat.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}
	if cmdResult.Handled {
		h.logger.Debug("Command handled in chat", "command", request.Message, "response", cmdResult.Message)
		response := chat.ChatResponse{
			Message:     cmdResult.Message,
			GameStateID: gs.ID,
			ChatHistory: gs.ChatHistory,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding chat response", "error", err)
		}
		return
	}

	messages, err := gs.GetChatMessages(cmdResult.Message, cmdResult.Role, scenario, PromptHistoryLimit)
	if err != nil {
		h.logger.Error("Error getting chat messages", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to get chat messages.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Generate response using LLM
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, err := h.llmService.Chat(ctx, messages)
	if err != nil {
		h.logger.Error("Error generating chat response",
			"error", err,
			"user_message", request.Message,
			"message_count", len(messages))
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse := ErrorResponse{
			Error: "Failed to generate response. Please try again.",
		}
		if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Cancel any in-process meta update for this game state
	h.metaCancelMu.Lock()
	if cancel, ok := h.metaCancel[gs.ID]; ok {
		cancel()
	}
	metaCtx, metaCancel := context.WithCancel(context.Background())
	h.metaCancel[gs.ID] = metaCancel
	h.metaCancelMu.Unlock()

	// Start background goroutine to update game meta (PromptState)
	go h.updateGameMeta(metaCtx, gs, request.Message, response.Message)

	// Exit early if the prompt is a system message
	if cmdResult.Role == chat.ChatRoleSystem {
		response.GameStateID = gs.ID
		//response.ChatHistory = gs.ChatHistory
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding chat response", "error", err)
		}
		return
	}

	// Update game state with new chat message
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    cmdResult.Role,
		Content: request.Message,
	})

	// Add the LLM's response to the game state
	response.Message = strings.TrimRight(response.Message, "\n")
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: response.Message,
	})

	// Save the updated game state
	if err := h.storage.SaveGameState(ctx, gs.ID, gs); err != nil {
		h.logger.Error("Failed to save game state", "error", err, "game_state_id", gs.ID.String())
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse := ErrorResponse{
			Error: "Failed to save conversation. Please try again.",
		}
		if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	response.GameStateID = gs.ID
	//response.ChatHistory = gs.ChatHistory
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Error encoding chat response", "error", err)
	}
}

var errSceneNotFound = errors.New("scene not found")

func applyMetaUpdate(gs *state.GameState, scenario *scenario.Scenario, metaUpdate *chat.MetaUpdate) error {
	if metaUpdate == nil {
		return nil
	}

	// Update Scene (invalid scene name fails silently for now)
	if metaUpdate.SceneName != "" && metaUpdate.SceneName != gs.SceneName && scenario.HasScene(metaUpdate.SceneName) {
		err := gs.LoadScene(scenario, metaUpdate.SceneName)
		if err != nil {
			return fmt.Errorf("failed to load scene: %w", err)
		}
		gs.SceneName = metaUpdate.SceneName
	}

	// Handle location change
	userLocationFound := false
	if metaUpdate.UserLocation != "" {
		// Loook for a location with this name in the game state
		for _, loc := range gs.WorldLocations {
			if loc.Name == metaUpdate.UserLocation {
				gs.Location = loc.Name
				userLocationFound = true
				break
			}
		}
		// If not found, do a best-effort match for world location
		// names as substrings of the user location
		if !userLocationFound {
			for _, loc := range gs.WorldLocations {
				if strings.Contains(strings.ToLower(metaUpdate.UserLocation), strings.ToLower(loc.Name)) {
					gs.Location = loc.Name
					break
				}
			}
		}
	}

	for _, item := range metaUpdate.AddToInventory {
		// add to inventory if not already present
		for _, invItem := range gs.Inventory {
			if invItem == item {
				continue
			}
		}
		// Item not found, add it
		if gs.Inventory == nil {
			gs.Inventory = make([]string, 0)
		}
		gs.Inventory = append(gs.Inventory, item)
	}

	for _, item := range metaUpdate.RemoveFromInventory {
		for i, invItem := range gs.Inventory {
			if invItem == item {
				gs.Inventory = append(gs.Inventory[:i], gs.Inventory[i+1:]...)
				break
			}
		}
	}

	for _, movedItem := range metaUpdate.MovedItems {
		fmt.Println("Processing moved item:", movedItem.Item, "from:", movedItem.From, "to:", movedItem.To)
		// Handle move FROM
		if movedItem.From != "" && movedItem.From != "user_inventory" {
			if loc, ok := gs.WorldLocations[movedItem.From]; ok {
				for i, invItem := range loc.Items {
					if invItem == movedItem.Item {
						loc.Items = append(loc.Items[:i], loc.Items[i+1:]...)
						gs.WorldLocations[movedItem.From] = loc // Write back
						break
					}
				}
			} else if npc, ok := gs.NPCs[movedItem.From]; ok {
				// If it's an NPC, remove the item from their inventory
				for i, invItem := range npc.Items {
					if invItem == movedItem.Item {
						npc.Items = append(npc.Items[:i], npc.Items[i+1:]...)
						gs.NPCs[movedItem.From] = npc // Write back
						break
					}
				}
			}
		}

		// Handle move TO
		if movedItem.To == "" || movedItem.To == "user_inventory" {
			continue
		}
		// check for a matching name in locations
		for key, loc := range gs.WorldLocations {
			fmt.Println("Checking location:", loc.Name, "for moved item:", movedItem.Item, "to:", movedItem.To)
			if loc.Name == movedItem.To {
				if loc.Items == nil {
					loc.Items = make([]string, 0)
				}
				loc.Items = append(loc.Items, movedItem.Item)
				gs.WorldLocations[key] = loc // Save the updated struct
				break
			}
		}
		if npc, ok := gs.NPCs[movedItem.To]; ok {
			if npc.Items == nil {
				npc.Items = make([]string, 0)
			}
			npc.Items = append(npc.Items, movedItem.Item)
			gs.NPCs[movedItem.To] = npc // Save the updated struct
		}
	}

	// Handle SetVars
	for k, v := range metaUpdate.SetVars {
		// Convert var name to lower case snake case
		snake := toSnakeCase(strings.ToLower(k))
		if gs.Vars == nil {
			gs.Vars = make(map[string]string)
		}
		gs.Vars[snake] = v
	}

	return nil
}

// toSnakeCase converts a string to lower snake_case
func toSnakeCase(s string) string {
	var out strings.Builder
	prevUnderscore := false
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			r = r + ('a' - 'A')
		}
		if r == ' ' || r == '-' || r == '.' {
			if !prevUnderscore && i > 0 {
				out.WriteRune('_')
				prevUnderscore = true
			}
			continue
		}
		if r == '_' {
			if !prevUnderscore && i > 0 {
				out.WriteRune('_')
				prevUnderscore = true
			}
			continue
		}
		out.WriteRune(r)
		prevUnderscore = false
	}
	return out.String()

	// TODO: NPC changes
}

// updateGameMeta runs in the background to extract and update the stateful parts
// of gamestate. This feels like the domain of gamestate. Might need to refactor.
func (h *ChatHandler) updateGameMeta(ctx context.Context, gs *state.GameState, userMessage string, responseMessage string) {
	start := time.Now()
	h.logger.Debug("Starting background game meta update", "game_state_id", gs.ID.String())
	defer func() {
		h.metaCancelMu.Lock()
		delete(h.metaCancel, gs.ID)
		h.metaCancelMu.Unlock()
	}()

	currentStateJSON, err := json.Marshal(state.ToBackgroundPromptState(gs))
	if err != nil {
		h.logger.Error("Failed to marshal current game state for meta update", "error", err, "game_state_id", gs.ID.String())
		return
	}

	s, err := h.storage.GetScenario(ctx, gs.Scenario)
	if err != nil {
		h.logger.Error("Failed to get scenario from storage", "error", err, "game_state_id", gs.ID.String())
		return
	}

	contingencyRules := make([]string, 0, len(s.ContingencyRules))
	if gs.SceneName != "" {
		contingencyRules = append(contingencyRules, s.Scenes[gs.SceneName].ContingencyRules...)
	}

	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: fmt.Sprintf(scenario.PromptStateExtractionInstructions, strings.Join(contingencyRules, "\n- ")),
		},
		{
			Role:    chat.ChatRoleSystem,
			Content: fmt.Sprintf("BEFORE game state: %s", string(currentStateJSON)),
		},
		{
			Role:    chat.ChatRoleUser,
			Content: userMessage,
		},
		{
			Role:    chat.ChatRoleAgent,
			Content: responseMessage,
		},
	}

	metaCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Send the meta update request to the LLM
	h.logger.Debug("Sending meta update request to LLM", "game_state_id", gs.ID.String(), "messages", messages)
	metaResponse, err := h.llmService.MetaUpdate(metaCtx, messages)
	if err != nil {
		h.logger.Error("Failed to get meta extraction response from LLM", "error", err, "game_state_id", gs.ID.String())
		return
	}
	if metaResponse == nil {
		return
	}

	latestGS, err := h.storage.LoadGameState(metaCtx, gs.ID)
	if err != nil {
		h.logger.Error("Failed to load latest game state for meta update", "error", err, "game_state_id", gs.ID.String())
		return
	}
	if latestGS == nil {
		h.logger.Warn("Game state not found during meta update", "game_state_id", gs.ID.String())
		return
	}

	// Apply the calculated state to the latest game state
	if err := applyMetaUpdate(latestGS, s, metaResponse); err != nil {
		if errors.Is(err, errSceneNotFound) {
			h.logger.Warn("Scene not found during meta update", "error", err, "game_state_id", latestGS.ID.String())
		} else {
			h.logger.Error("Failed to apply meta update", "error", err, "game_state_id", latestGS.ID.String())
			return
		}
	}

	// Save the updated game state
	if err := h.storage.SaveGameState(metaCtx, latestGS.ID, latestGS); err != nil {
		h.logger.Error("Failed to save updated game state after meta extraction", "error", err, "game_state_id", latestGS.ID.String())
		return
	}

	h.logger.Debug("Successfully updated game meta",
		"game_state_id", gs.ID.String(),
		"meta_response", metaResponse,
		"duration_s", time.Since(start).Seconds())
}
