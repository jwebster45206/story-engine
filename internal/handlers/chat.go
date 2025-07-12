package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jwebster45206/roleplay-agent/internal/services"
	"github.com/jwebster45206/roleplay-agent/pkg/chat"
	"github.com/jwebster45206/roleplay-agent/pkg/scenario"
)

// ChatHandler handles chat requests
type ChatHandler struct {
	llmService services.LLMService
	logger     *slog.Logger
}

// NewChatHandler creates a new chat handler
func NewChatHandler(llmService services.LLMService, logger *slog.Logger) *ChatHandler {
	return &ChatHandler{
		llmService: llmService,
		logger:     logger,
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
		response := chat.ChatResponse{
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

	h.logger.Info("Chat endpoint accessed",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr)

	// Parse request body
	var request chat.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Warn("Invalid request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		response := chat.ChatResponse{
			Error: "Invalid request body. Expected JSON with 'message' field.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Validate request
	if request.Message == "" {
		h.logger.Warn("Empty message in request")
		w.WriteHeader(http.StatusBadRequest)
		response := chat.ChatResponse{
			Error: "Message cannot be empty.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Create chat message
	messages := []chat.ChatMessage{
		{
			Role:    chat.ChatRoleSystem,
			Content: scenario.BaseSystemPrompt,
		},
		{
			Role:    chat.ChatRoleUser,
			Content: request.Message + " " + scenario.LocationPrompt,
		},
	}

	// Generate response using LLM service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := h.llmService.GetChatResponse(ctx, messages)
	if err != nil {
		h.logger.Error("Error generating chat response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse := chat.ChatResponse{
			Error: "Failed to generate response. Please try again.",
		}
		if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	// Return successful response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Error encoding chat response", "error", err)
	}
}
