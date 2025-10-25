package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/queue"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// ChatHandler handles chat HTTP requests by enqueuing them for async processing
type ChatHandler struct {
	chatQueue state.ChatQueue
	logger    *slog.Logger
}

// NewChatHandler creates a new chat handler
func NewChatHandler(chatQueue state.ChatQueue, logger *slog.Logger) *ChatHandler {
	return &ChatHandler{
		chatQueue: chatQueue,
		logger:    logger,
	}
}

// ChatResponse is the response format for async chat requests
type ChatResponse struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
}

// ServeHTTP handles HTTP requests for chat by enqueuing them for async processing
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

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

	// Create queue request
	requestID := uuid.New().String()
	queueReq := &queue.Request{
		RequestID:   requestID,
		Type:        queue.RequestTypeChat,
		GameStateID: request.GameStateID,
		Message:     request.Message,
		EnqueuedAt:  time.Now(),
	}

	// Enqueue for async processing
	if err := h.chatQueue.EnqueueRequest(r.Context(), queueReq); err != nil {
		h.logger.Error("Failed to enqueue chat request", "error", err, "request_id", requestID)
		w.WriteHeader(http.StatusInternalServerError)
		response := ErrorResponse{
			Error: "Failed to enqueue request for processing.",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Error encoding error response", "error", err)
		}
		return
	}

	h.logger.Info("Chat request enqueued",
		"request_id", requestID,
		"game_state_id", request.GameStateID.String())

	// Return request ID for client to poll status
	w.WriteHeader(http.StatusAccepted)
	response := ChatResponse{
		RequestID: requestID,
		Message:   "Request accepted for processing. Poll game state for updates.",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Error encoding chat response", "error", err)
	}
}
