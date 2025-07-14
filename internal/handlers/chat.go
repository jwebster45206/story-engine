package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/roleplay-agent/internal/services"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
	"github.com/jwebster45206/roleplay-agent/pkg/scenario"
	"github.com/jwebster45206/roleplay-agent/pkg/state"
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

// ServeHTTP handles HTTP requests for chat
// TODO:
//   - Load the gamestate (chat history) from Redis by UUID
//   - Create system prompt using
//   - Construct the Ollama chat prompt by combining the gamestate (just chat history
//     for now), user message, system prompt, and character description.
//   - Call the LLM service to generate a response
//   - Save updated gamestate
//   - Return the response as JSON
//
// Next steps: Add redis to docker compose, and add redis client to the service layer.
// Refine prompt construction, based on both gameplay requirements and LLM capabilities.
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow POST method
	if r.Method != http.MethodPost {
		h.logger.Warn("Method not allowed for chat endpoint",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr)

		w.WriteHeader(http.StatusMethodNotAllowed)
		response := ErrorResponse{
			Error: "Method not allowed. Only POST is supported.",
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

	// Parse request body
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

	if request.GameStateID == uuid.Nil {
		w.WriteHeader(http.StatusBadRequest)
		response := ErrorResponse{
			Error: "Game state ID is required.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Load existing game state from Redis
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

	// Translate game state to a chat prompt
	statePrompt, err := gs.GetStatePrompt()
	if err != nil {
		h.logger.Error("Error generating state prompt", "error", err, "game_state_id", gs.ID.String())
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to generate state prompt. ",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// System prompt first
	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: scenario.BaseSystemPrompt + "\n\n" + scenario.PirateScenarioPrompt,
		},
		statePrompt, // game state context json
	}

	// Add chat history from game state
	messages = append(messages, gs.GetHistoryForPrompt()...)
	messages = append(messages, chat.ChatMessage{
		Role:    chat.ChatRoleUser,
		Content: request.Message,
	})
	// Instructions about how to respond to user input
	messages = append(messages, gs.GetClosingPrompt())

	// Generate response using LLM
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, err := h.llmService.GetChatResponse(ctx, messages)
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

	// Update game state with new chat message
	gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
		Role:    chat.ChatRoleUser,
		Content: request.Message,
	})
	// Add the LLM's response to the game state
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

	// Cancel any in-process meta update for this game state
	h.metaCancelMu.Lock()
	if cancel, ok := h.metaCancel[gs.ID]; ok {
		cancel()
	}
	metaCtx, metaCancel := context.WithCancel(context.Background())
	h.metaCancel[gs.ID] = metaCancel
	h.metaCancelMu.Unlock()

	// Start background goroutine to update game meta (PromptState)
	go h.updateGameMeta(metaCtx, gs, request, response)

	response.GameStateID = gs.ID
	response.ChatHistory = gs.ChatHistory
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Error encoding chat response", "error", err)
	}
}

// updateGameMeta runs in the background to extract and update game metadata
func (h *ChatHandler) updateGameMeta(ctx context.Context, gs *state.GameState, request chat.ChatRequest, response *chat.ChatResponse) {
	h.logger.Debug("Starting background game meta update", "game_state_id", gs.ID.String())
	defer func() {
		h.metaCancelMu.Lock()
		if _, ok := h.metaCancel[gs.ID]; ok && ctx.Err() == nil {
			delete(h.metaCancel, gs.ID)
		}
		h.metaCancelMu.Unlock()
	}()

	// Create messages for the meta extraction request
	currentStateJSON, err := json.Marshal(state.ToPromptState(gs))
	if err != nil {
		h.logger.Error("Failed to marshal current game state for meta update", "error", err, "game_state_id", gs.ID.String())
		return
	}

	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: scenario.PromptStateExtractionInstructions,
		},
		{
			Role:    chat.ChatRoleSystem,
			Content: fmt.Sprintf("Current game state: %s", string(currentStateJSON)),
		},
		{
			Role:    chat.ChatRoleUser,
			Content: request.Message,
		},
		{
			Role:    chat.ChatRoleAgent,
			Content: response.Message,
		},
	}

	// Create a timeout context for the meta update
	metaCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Send the extraction request to the LLM
	metaResponse, err := h.llmService.GetChatResponse(metaCtx, messages)
	if err != nil {
		h.logger.Error("Failed to get meta extraction response from LLM", "error", err, "game_state_id", gs.ID.String())
		return
	}

	// Parse the JSON response
	var ps state.PromptState
	if err := json.Unmarshal([]byte(metaResponse.Message), &ps); err != nil {
		h.logger.Error("Failed to unmarshal meta extraction JSON", "error", err, "response", metaResponse.Message, "game_state_id", gs.ID.String())
		return
	}

	// Load the latest game state (to avoid race conditions)
	latestGS, err := h.storage.LoadGameState(metaCtx, gs.ID)
	if err != nil {
		h.logger.Error("Failed to load latest game state for meta update", "error", err, "game_state_id", gs.ID.String())
		return
	}

	if latestGS == nil {
		h.logger.Warn("Game state not found during meta update", "game_state_id", gs.ID.String())
		return
	}

	// Apply the extracted state to the latest game state
	state.ApplyPromptStateToGameState(&ps, latestGS)

	// Save the updated game state
	if err := h.storage.SaveGameState(metaCtx, latestGS.ID, latestGS); err != nil {
		h.logger.Error("Failed to save updated game state after meta extraction", "error", err, "game_state_id", latestGS.ID.String())
		return
	}

	h.logger.Debug("Successfully updated game meta", "game_state_id", gs.ID.String(), "prompt_state", ps)
}
