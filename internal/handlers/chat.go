package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
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
}

// NewChatHandler creates a new chat handler
func NewChatHandler(llmService services.LLMService, logger *slog.Logger, storage services.Storage) *ChatHandler {
	return &ChatHandler{
		llmService: llmService,
		logger:     logger,
		storage:    storage,
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
		statePrompt,
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

	// Attempt to extract and apply Gamestate JSON from the LLM response
	if err := parseAndApplyGameState(&response.Message, gs, h.logger); err != nil {
		h.logger.Error("Failed to parse Gamestate JSON from LLM response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse := ErrorResponse{
			Error: "Failed to parse Gamestate JSON from LLM response.",
		}
		_ = json.NewEncoder(w).Encode(errorResponse)
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

	response.GameStateID = gs.ID
	response.ChatHistory = gs.ChatHistory
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Error encoding chat response", "error", err)
	}
}

// parseAndApplyGameState looks for a Gamestate JSON block in the response message, applies it to the game state, and removes it from the message.
func parseAndApplyGameState(responseMsg *string, gs *state.GameState, log *slog.Logger) error {
	msg := *responseMsg
	prefix := "Gamestate:"
	prefixIdx := strings.Index(msg, prefix)
	// Accept both ```json, ```JSON, and plain ``` (case-insensitive)
	codeIdx := strings.Index(strings.ToLower(msg), "```json")
	codeMarker := "```json"
	if codeIdx == -1 {
		codeIdx = strings.Index(msg, "```")
		codeMarker = "```"
	}
	if codeIdx == -1 {
		log.Debug("No code block found in response", "msg", msg)
		return nil // No code block, nothing to do
	}

	var blockStart int
	if prefixIdx != -1 && prefixIdx < codeIdx {
		blockStart = prefixIdx
	} else {
		blockStart = codeIdx
	}
	// Find the end of the code block
	codeEnd := strings.Index(msg[codeIdx+len(codeMarker):], "```")
	if codeEnd == -1 {
		log.Debug("No closing code block found after code marker", "msg", msg[codeIdx:])
		// Remove everything from blockStart to end of message
		cleaned := strings.TrimSpace(msg[:blockStart])
		*responseMsg = cleaned
		return nil
	}

	codeEnd = codeIdx + len(codeMarker) + codeEnd
	jsonStr := strings.TrimSpace(msg[codeIdx+len(codeMarker) : codeEnd])
	// If the block starts with '"gamestate":', wrap it in braces to make valid JSON
	trimmed := strings.TrimSpace(jsonStr)
	if strings.HasPrefix(trimmed, "\"gamestate\"") || strings.HasPrefix(trimmed, "'gamestate'") {
		jsonStr = "{" + trimmed + "}"
	} else if strings.HasPrefix(trimmed, "gamestate:") {
		// Remove "gamestate:" and wrap in {"gamestate": ...}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "gamestate:"))
		jsonStr = "{\"gamestate\": " + rest + "}"
	}
	log.Debug("Found Gamestate JSON block in response",
		"gamestate_json", jsonStr,
		"game_state_id", gs.ID.String(),
	)
	var ps state.PromptState
	if err := json.Unmarshal([]byte(jsonStr), &ps); err != nil {
		log.Debug("Failed to unmarshal Gamestate JSON", "jsonStr", jsonStr, "error", err)
		return err
	}
	state.ApplyPromptStateToGameState(&ps, gs)

	// Remove everything from blockStart to end of message
	cleaned := strings.TrimSpace(msg[:blockStart])
	*responseMsg = cleaned
	return nil
}
