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

	// For background gamestate delta cancellation
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

	if request.Stream {
		h.handleStreamChat(w, r, request)
	} else {
		h.handleRestChat(w, r, request)
	}
}

// handleRestChat handles non-streaming chat requests
func (h *ChatHandler) handleRestChat(w http.ResponseWriter, r *http.Request, request chat.ChatRequest) {
	w.Header().Set("Content-Type", "application/json")

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h.logger.Debug("Sending chat request to LLM", "game_state_id", gs.ID.String(), "messages", messages)
	response, err := h.llmService.Chat(ctx, messages)
	if err != nil {
		h.logger.Error("Error generating chat response",
			"error", err,
			"user_message", request.Message,
			"message_count", len(messages))
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse := ErrorResponse{
			Error: "Failed to generate response. Internal error.",
		}
		if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Cancel any in-process gamestate delta for this game state
	h.metaCancelMu.Lock()
	if cancel, ok := h.metaCancel[gs.ID]; ok {
		cancel()
	}
	metaCtx, metaCancel := context.WithCancel(context.Background())
	h.metaCancel[gs.ID] = metaCancel
	h.metaCancelMu.Unlock()

	if !gs.IsEnded {
		// Update turn counters before background updates
		gs.IncrementTurnCounters()
		// Start background goroutine to update game meta (PromptState)
		go h.syncGameState(metaCtx, gs, request.Message, response.Message)
	}

	// Exit early if the prompt is a system message
	if cmdResult.Role == chat.ChatRoleSystem {
		response.GameStateID = gs.ID
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
			Error: "Failed to save conversation. Internal error.",
		}
		if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	response.GameStateID = gs.ID
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Error encoding chat response", "error", err)
	}
}

// handleStreamChat handles streaming chat requests
func (h *ChatHandler) handleStreamChat(w http.ResponseWriter, r *http.Request, request chat.ChatRequest) {
	// Load game state
	gs, err := h.storage.LoadGameState(r.Context(), request.GameStateID)
	if err != nil {
		h.logger.Error("Error loading game state", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to load game state.",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if gs == nil {
		h.logger.Warn("Game state not found", "requested_id", request.GameStateID.String())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Game state not found. Please provide a valid game state ID.",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get Scenario for the chat
	scenario, err := h.storage.GetScenario(r.Context(), gs.Scenario)
	if err != nil {
		h.logger.Error("Error loading scenario for chat", "error", err, "scenario_filename", gs.Scenario)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to load scenario for chat.",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	cmdResult, err := gs.TryHandleCommand(request.Message)
	if err != nil {
		h.logger.Error("Error handling command in chat", "error", err, "command", request.Message)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to handle command in chat.",
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	// Handle commands before streaming setup
	if cmdResult.Handled {
		h.logger.Debug("Command handled in chat", "command", request.Message, "response", cmdResult.Message)
		// For commands, we can still stream the response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.sendSSEChunk(w, services.StreamChunk{Content: cmdResult.Message, Done: true})
		return
	}

	messages, err := gs.GetChatMessages(cmdResult.Message, cmdResult.Role, scenario, PromptHistoryLimit)
	if err != nil {
		h.logger.Error("Error getting chat messages", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to get chat messages.",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Initialize LLM streaming (final validation step before committing to SSE)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h.logger.Debug("Sending streaming chat request to LLM", "game_state_id", gs.ID.String(), "messages", messages)
	streamChan, err := h.llmService.ChatStream(ctx, messages)
	if err != nil {
		h.logger.Error("Error generating streaming chat response",
			"error", err,
			"user_message", request.Message,
			"message_count", len(messages))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to generate response. Internal Error.",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// ONLY NOW set SSE headers - ALL validation passed including LLM initialization
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Stream the response
	var fullResponse strings.Builder
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("Streaming not supported")
		h.sendSSEError(w, "Streaming not supported by this server.")
		return
	}

	for chunk := range streamChan {
		select {
		case <-r.Context().Done():
			h.logger.Debug("Client disconnected during streaming")
			return
		default:
		}

		if chunk.Error != nil {
			h.logger.Error("Error in streaming response", "error", chunk.Error)
			h.sendSSEError(w, "Error generating response.")
			return
		}

		// Send the chunk
		h.sendSSEChunk(w, chunk)
		flusher.Flush()

		// Accumulate content for game state update
		if chunk.Content != "" {
			fullResponse.WriteString(chunk.Content)
		}

		if chunk.Done {
			// Start background game state update
			go h.updateGameStateAfterStreaming(gs, request.Message, fullResponse.String(), cmdResult.Role)
			return
		}
	}
}

// sendSSEChunk sends a streaming chunk in SSE format
func (h *ChatHandler) sendSSEChunk(w http.ResponseWriter, chunk services.StreamChunk) {
	data, err := json.Marshal(chunk)
	if err != nil {
		h.logger.Error("Error marshaling SSE chunk", "error", err)
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

// sendSSEError sends an error in SSE format
func (h *ChatHandler) sendSSEError(w http.ResponseWriter, message string) {
	errorChunk := services.StreamChunk{
		Error: errors.New(message),
		Done:  true,
	}
	h.sendSSEChunk(w, errorChunk)
}

// updateGameStateAfterStreaming updates game state after streaming is complete
func (h *ChatHandler) updateGameStateAfterStreaming(gs *state.GameState, userMessage, responseMessage, userRole string) {
	ctx := context.Background()

	// Cancel any in-process gamestate delta for this game state
	h.metaCancelMu.Lock()
	if cancel, ok := h.metaCancel[gs.ID]; ok {
		cancel()
	}
	metaCtx, metaCancel := context.WithCancel(context.Background())
	h.metaCancel[gs.ID] = metaCancel
	h.metaCancelMu.Unlock()

	if !gs.IsEnded {
		gs.IncrementTurnCounters()
	}

	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    userRole,
		Content: userMessage,
	})

	responseMessage = strings.TrimRight(responseMessage, "\n")
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: responseMessage,
	})

	if err := h.storage.SaveGameState(ctx, gs.ID, gs); err != nil {
		h.logger.Error("Failed to save game state after streaming", "error", err, "game_state_id", gs.ID.String())
		return
	}

	// Start background gamestate delta update if game is not ended
	if !gs.IsEnded {
		go h.syncGameState(metaCtx, gs, userMessage, responseMessage)
	}

	h.logger.Debug("Game state updated after streaming", "game_state_id", gs.ID.String())
}

var errSceneNotFound = errors.New("scene not found")

// applyGameStateDelta applies a GameStateDelta to the given GameState.
func applyGameStateDelta(gs *state.GameState, scenario *scenario.Scenario, delta *state.GameStateDelta) error {
	if delta == nil {
		return nil
	}

	// Handle scene change
	if delta.SceneChange != nil && delta.SceneChange.To != "" && delta.SceneChange.To != gs.SceneName && scenario.HasScene(delta.SceneChange.To) {
		err := gs.LoadScene(scenario, delta.SceneChange.To)
		if err != nil {
			return fmt.Errorf("failed to load scene: %w", err)
		}
		gs.SceneName = delta.SceneChange.To
	}

	// Handle location change
	userLocationFound := false
	if delta.UserLocation != "" {
		// Look for a location with this name in the game state
		for _, loc := range gs.WorldLocations {
			if loc.Name == delta.UserLocation {
				gs.Location = loc.Name
				userLocationFound = true
				break
			}
		}
		// If not found, do a best-effort match for world location
		// names as substrings of the user location
		if !userLocationFound {
			for _, loc := range gs.WorldLocations {
				if strings.Contains(strings.ToLower(delta.UserLocation), strings.ToLower(loc.Name)) {
					gs.Location = loc.Name
					break
				}
			}
		}
	}

	// Handle item events
	for _, itemEvent := range delta.ItemEvents {
		switch itemEvent.Action {
		case "acquire":
			// Add item to player inventory
			itemExists := false
			for _, invItem := range gs.Inventory {
				if invItem == itemEvent.Item {
					itemExists = true
					break
				}
			}
			if !itemExists {
				if gs.Inventory == nil {
					gs.Inventory = make([]string, 0)
				}
				gs.Inventory = append(gs.Inventory, itemEvent.Item)
			}
			// Remove from source if specified and not consumed
			if itemEvent.From != nil && (itemEvent.Consumed == nil || !*itemEvent.Consumed) {
				removeItemFromSource(gs, itemEvent.Item, itemEvent.From)
			}

		case "drop":
			// Remove from player inventory
			for i, invItem := range gs.Inventory {
				if invItem == itemEvent.Item {
					gs.Inventory = append(gs.Inventory[:i], gs.Inventory[i+1:]...)
					break
				}
			}
			// Add to destination if specified
			if itemEvent.To != nil {
				addItemToDestination(gs, itemEvent.Item, itemEvent.To)
			}

		case "give":
			// Remove from source
			if itemEvent.From != nil {
				removeItemFromSource(gs, itemEvent.Item, itemEvent.From)
			} else {
				// Default to removing from player inventory if no source specified
				for i, invItem := range gs.Inventory {
					if invItem == itemEvent.Item {
						gs.Inventory = append(gs.Inventory[:i], gs.Inventory[i+1:]...)
						break
					}
				}
			}
			// Add to destination
			if itemEvent.To != nil {
				addItemToDestination(gs, itemEvent.Item, itemEvent.To)
			}

		case "move":
			// Remove from source
			if itemEvent.From != nil {
				removeItemFromSource(gs, itemEvent.Item, itemEvent.From)
			}
			// Add to destination
			if itemEvent.To != nil {
				addItemToDestination(gs, itemEvent.Item, itemEvent.To)
			}

		case "use":
			// If item is consumed, remove it from source
			if itemEvent.Consumed != nil && *itemEvent.Consumed {
				if itemEvent.From != nil {
					removeItemFromSource(gs, itemEvent.Item, itemEvent.From)
				} else {
					// Default to removing from player inventory if no source specified
					for i, invItem := range gs.Inventory {
						if invItem == itemEvent.Item {
							gs.Inventory = append(gs.Inventory[:i], gs.Inventory[i+1:]...)
							break
						}
					}
				}
			}
		}
	}

	// Handle SetVars
	for k, v := range delta.SetVars {
		// Convert var name to lower case snake case
		snake := toSnakeCase(strings.ToLower(k))
		if gs.Vars == nil {
			gs.Vars = make(map[string]string)
		}
		gs.Vars[snake] = v
	}

	// Handle Game End
	if delta.GameEnded != nil && *delta.GameEnded {
		gs.IsEnded = true
	}

	// Ensure that items are singletons
	gs.NormalizeItems()

	return nil
}

// removeItemFromSource removes an item from the specified source
func removeItemFromSource(gs *state.GameState, item string, from *struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}) {
	switch from.Type {
	case "player":
		// Remove from player inventory
		for i, invItem := range gs.Inventory {
			if invItem == item {
				gs.Inventory = append(gs.Inventory[:i], gs.Inventory[i+1:]...)
				break
			}
		}
	case "location":
		// Remove from location
		for key, loc := range gs.WorldLocations {
			if loc.Name == from.Name {
				for i, invItem := range loc.Items {
					if invItem == item {
						loc.Items = append(loc.Items[:i], loc.Items[i+1:]...)
						gs.WorldLocations[key] = loc // Write back
						break
					}
				}
				break
			}
		}
	case "npc":
		// Remove from NPC
		if npc, ok := gs.NPCs[from.Name]; ok {
			for i, invItem := range npc.Items {
				if invItem == item {
					npc.Items = append(npc.Items[:i], npc.Items[i+1:]...)
					gs.NPCs[from.Name] = npc // Write back
					break
				}
			}
		}
	}
}

// addItemToDestination adds an item to the specified destination
func addItemToDestination(gs *state.GameState, item string, to *struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}) {
	switch to.Type {
	case "player":
		// Add to player inventory (check for duplicates)
		itemExists := false
		for _, invItem := range gs.Inventory {
			if invItem == item {
				itemExists = true
				break
			}
		}
		if !itemExists {
			if gs.Inventory == nil {
				gs.Inventory = make([]string, 0)
			}
			gs.Inventory = append(gs.Inventory, item)
		}
	case "location":
		// Add to location
		for key, loc := range gs.WorldLocations {
			if loc.Name == to.Name {
				if loc.Items == nil {
					loc.Items = make([]string, 0)
				}
				loc.Items = append(loc.Items, item)
				gs.WorldLocations[key] = loc // Write back
				break
			}
		}
	case "npc":
		// Add to NPC
		if npc, ok := gs.NPCs[to.Name]; ok {
			if npc.Items == nil {
				npc.Items = make([]string, 0)
			}
			npc.Items = append(npc.Items, item)
			gs.NPCs[to.Name] = npc // Write back
		}
	}
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
}

// syncGameState runs in the background to extract and update the stateful parts
// of gamestate.
func (h *ChatHandler) syncGameState(ctx context.Context, gs *state.GameState, userMessage string, responseMessage string) {
	start := time.Now()
	h.logger.Debug("Starting background game gamestate delta", "game_state_id", gs.ID.String())
	defer func() {
		h.metaCancelMu.Lock()
		delete(h.metaCancel, gs.ID)
		h.metaCancelMu.Unlock()
	}()

	currentStateJSON, err := json.Marshal(state.ToBackgroundPromptState(gs))
	if err != nil {
		h.logger.Error("Failed to marshal current game state for gamestate delta", "error", err, "game_state_id", gs.ID.String())
		return
	}

	s, err := h.storage.GetScenario(ctx, gs.Scenario)
	if err != nil {
		h.logger.Error("Failed to get scenario from storage", "error", err, "game_state_id", gs.ID.String())
		return
	}

	contingencyRules := scenario.GlobalContingencyRules
	contingencyRules = append(contingencyRules, s.ContingencyRules...)
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

	// Send the gamestate delta request to the LLM
	h.logger.Debug("Sending gamestate delta request to LLM", "game_state_id", gs.ID.String(), "messages", messages)
	delta, backendModel, err := h.llmService.DeltaUpdate(metaCtx, messages)
	if err != nil {
		h.logger.Error("Failed to get meta extraction response from LLM", "error", err, "game_state_id", gs.ID.String())
		return
	}
	h.logger.Debug("Received gamestate delta from LLM", "game_state_id", gs.ID.String(), "delta", delta, "backend_model", backendModel)
	if delta == nil {
		return
	}

	latestGS, err := h.storage.LoadGameState(metaCtx, gs.ID)
	if err != nil {
		h.logger.Error("Failed to load latest game state for gamestate delta", "error", err, "game_state_id", gs.ID.String())
		return
	}
	if latestGS == nil {
		h.logger.Warn("Game state not found during gamestate delta", "game_state_id", gs.ID.String())
		return
	}

	// Apply the calculated state to the latest game state
	if err := applyGameStateDelta(latestGS, s, delta); err != nil {
		if errors.Is(err, errSceneNotFound) {
			h.logger.Warn("Scene not found during applyGameStateDelta", "error", err, "game_state_id", latestGS.ID.String())
		} else {
			h.logger.Error("Failed applyGameStateDelta", "error", err, "game_state_id", latestGS.ID.String())
			return
		}
	}

	// Save the updated game state
	if err := h.storage.SaveGameState(metaCtx, latestGS.ID, latestGS); err != nil {
		h.logger.Error("Failed to save updated game state after meta extraction", "error", err, "game_state_id", latestGS.ID.String())
		return
	}

	h.logger.Debug("Updated game meta",
		"game_state_id", gs.ID.String(),
		"delta", delta,
		"duration_s", time.Since(start).Seconds(),
		"backend_model", backendModel,
	)
}
