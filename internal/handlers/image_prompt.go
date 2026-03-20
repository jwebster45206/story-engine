package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/prompts"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

const imagePromptTimeout = 60 * time.Second

// ImagePromptHandler handles POST /v1/image-prompt requests.
type ImagePromptHandler struct {
	storage    storage.Storage
	llmService services.LLMService
	logger     *slog.Logger
}

// NewImagePromptHandler creates a new ImagePromptHandler.
func NewImagePromptHandler(storage storage.Storage, llmService services.LLMService, logger *slog.Logger) *ImagePromptHandler {
	return &ImagePromptHandler{
		storage:    storage,
		llmService: llmService,
		logger:     logger,
	}
}

// ServeHTTP handles HTTP requests for the image-prompt endpoint.
// Only POST /v1/image-prompt is accepted.
func (h *ImagePromptHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/v1/image-prompt") {
		h.logger.Warn("Method not allowed for image-prompt endpoint",
			"method", r.Method,
			"path", r.URL.Path)
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Method not allowed. Only POST is supported at /v1/image-prompt.",
		}); err != nil {
			h.logger.Error("Error encoding method-not-allowed response", "error", err)
		}
		return
	}

	var req chat.ImagePromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid request body for image-prompt", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Invalid request body. Expected JSON with 'gamestate_id' field.",
		}); err != nil {
			h.logger.Error("Error encoding bad-request response", "error", err)
		}
		return
	}

	if err := req.Validate(); err != nil {
		h.logger.Warn("Invalid image-prompt request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		}); err != nil {
			h.logger.Error("Error encoding bad-request response", "error", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), imagePromptTimeout)
	defer cancel()

	// Load game state
	gs, err := h.storage.LoadGameState(ctx, req.GameStateID)
	if err != nil {
		h.logger.Warn("Game state not found for image-prompt",
			"gamestate_id", req.GameStateID, "error", err)
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Game state not found.",
		}); err != nil {
			h.logger.Error("Error encoding not-found response", "error", err)
		}
		return
	}

	// Load scenario
	scenario, err := h.storage.GetScenario(ctx, gs.Scenario)
	if err != nil {
		h.logger.Error("Failed to load scenario for image-prompt",
			"scenario", gs.Scenario, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Failed to load scenario.",
		}); err != nil {
			h.logger.Error("Error encoding internal-error response", "error", err)
		}
		return
	}

	// Build the LLM messages
	messages, err := prompts.BuildImagePromptMessages(gs, scenario)
	if err != nil {
		h.logger.Warn("Cannot build image prompt messages", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: err.Error(),
		}); err != nil {
			h.logger.Error("Error encoding bad-request response", "error", err)
		}
		return
	}

	// Call the LLM
	resp, err := h.llmService.Chat(ctx, messages)
	if err != nil {
		h.logger.Error("LLM call failed for image-prompt",
			"gamestate_id", req.GameStateID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Failed to generate image prompt.",
		}); err != nil {
			h.logger.Error("Error encoding internal-error response", "error", err)
		}
		return
	}

	h.logger.Info("Image prompt generated",
		"gamestate_id", req.GameStateID,
		"prompt_len", len(resp.Message))

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(chat.ImagePromptResponse{
		GameStateID: req.GameStateID,
		Prompt:      resp.Message,
	}); err != nil {
		h.logger.Error("Error encoding image-prompt response", "error", err)
	}
}
