package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jwebster45206/story-engine/internal/services"
)

type HealthResponse struct {
	Status     string                 `json:"status"`
	Timestamp  time.Time              `json:"timestamp"`
	Service    string                 `json:"service"`
	Components map[string]interface{} `json:"components"`
}

type HealthHandler struct {
	storage    services.Storage
	llmService services.LLMService
	logger     *slog.Logger
}

func NewHealthHandler(storage services.Storage, llmService services.LLMService, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		storage:    storage,
		llmService: llmService,
		logger:     logger,
	}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Debug("Health check requested",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr)

	// Check cache health
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	components := make(map[string]interface{})
	overallStatus := "healthy"

	// Test storage connection
	if err := h.storage.Ping(ctx); err != nil {
		h.logger.Warn("Storage health check failed", "error", err)
		components["storage"] = "unhealthy"
		overallStatus = "degraded"
	} else {
		components["storage"] = "healthy"
	}

	response := HealthResponse{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Service:    "story-engine",
		Components: components,
	}

	statusCode := http.StatusOK
	if overallStatus != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Error encoding health response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
