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

	// Check for queued story events
	storyEventPrompt := gs.GetStoryEvents()
	if storyEventPrompt != "" {
		gs.ClearStoryEventQueue()
		h.logger.Debug("Story events will be injected", "game_state_id", gs.ID.String(), "events", storyEventPrompt)
	}

	messages, err := gs.GetChatMessages(cmdResult.Message, cmdResult.Role, scenario, PromptHistoryLimit, storyEventPrompt)
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
		// Make a deep copy for the background goroutine to avoid data races
		gsCopy, err := gs.DeepCopy()
		if err != nil {
			h.logger.Error("Failed to copy game state for background sync", "error", err, "game_state_id", gs.ID.String())
		} else {
			// Start background goroutine to update game meta (PromptState)
			go h.syncGameState(metaCtx, gsCopy, request.Message, response.Message, storyEventPrompt)
		}
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
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	if gs == nil {
		h.logger.Warn("Game state not found", "requested_id", request.GameStateID.String())
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to handle command in chat.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
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

	// Check for queued story events
	storyEventPrompt := gs.GetStoryEvents()
	if storyEventPrompt != "" {
		gs.ClearStoryEventQueue()
		h.logger.Debug("Story events will be injected", "game_state_id", gs.ID.String(), "events", storyEventPrompt)
	}

	messages, err := gs.GetChatMessages(cmdResult.Message, cmdResult.Role, scenario, PromptHistoryLimit, storyEventPrompt)
	if err != nil {
		h.logger.Error("Error getting chat messages", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to get chat messages.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
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
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
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
			go h.updateGameStateAfterStreaming(gs, request.Message, fullResponse.String(), cmdResult.Role, storyEventPrompt)
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
func (h *ChatHandler) updateGameStateAfterStreaming(gs *state.GameState, userMessage, responseMessage, userRole, storyEventPrompt string) {
	ctx := context.Background()

	// Cancel any in-process gamestate delta for this game state
	h.metaCancelMu.Lock()
	if cancel, ok := h.metaCancel[gs.ID]; ok {
		cancel()
	}
	metaCtx, metaCancel := context.WithCancel(context.Background())
	h.metaCancel[gs.ID] = metaCancel
	h.metaCancelMu.Unlock()

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
		go h.syncGameState(metaCtx, gs, userMessage, responseMessage, storyEventPrompt)
	}

	h.logger.Debug("Game state updated after streaming", "game_state_id", gs.ID.String())
}

// syncGameState runs in the background to extract and update the stateful parts
// of gamestate.
func (h *ChatHandler) syncGameState(ctx context.Context, gs *state.GameState, userMessage string, responseMessage string, storyEventPrompt string) {
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
	}

	// Add story event message if it exists
	if storyEventPrompt != "" {
		messages = append(messages, chat.ChatMessage{
			Role:    chat.ChatRoleSystem,
			Content: storyEventPrompt,
		})
	}

	// Add the narrator response
	messages = append(messages, chat.ChatMessage{
		Role:    chat.ChatRoleAgent,
		Content: responseMessage,
	})

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

	// Increment turn counters on the latest game state
	if !latestGS.IsEnded {
		latestGS.IncrementTurnCounters()
	}

	// Use DeltaWorker to handle all delta application logic
	worker := state.NewDeltaWorker(latestGS, delta, s)

	// Apply vars first (before evaluating conditionals)
	worker.ApplyVars()

	// Evaluate conditionals and override delta based on results
	triggeredConditionals := worker.ApplyConditionalOverrides()

	// Log triggered conditionals
	if len(triggeredConditionals) > 0 {
		for _, conditional := range triggeredConditionals {
			condName := conditional.Name
			if condName == "" {
				condName = "(unnamed)"
			}
			if conditional.Then.Scene != "" {
				h.logger.Info("Conditional scene change", "game_state_id", latestGS.ID.String(), "name", condName, "to_scene", conditional.Then.Scene)
			}
			if conditional.Then.GameEnded != nil {
				h.logger.Info("Conditional game ended", "game_state_id", latestGS.ID.String(), "name", condName, "ended", *conditional.Then.GameEnded)
			}
		}
	}

	// Queue story events for next turn
	triggeredEvents := worker.QueueStoryEvents()
	if len(triggeredEvents) > 0 {
		for _, event := range triggeredEvents {
			previewLen := 50
			if len(event.Prompt) < previewLen {
				previewLen = len(event.Prompt)
			}
			h.logger.Info("Story event queued", "game_state_id", latestGS.ID.String(), "name", event.Name, "prompt_preview", event.Prompt[:previewLen]+"...")
		}
	}

	// Apply the final delta to the game state
	if err := worker.Apply(); err != nil {
		h.logger.Error("Failed to apply delta", "error", err, "game_state_id", latestGS.ID.String())
		return
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
