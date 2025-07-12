package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jwebster45206/roleplay-agent/internal/services"
)

type HealthResponse struct {
	Status     string                 `json:"status"`
	Timestamp  time.Time              `json:"timestamp"`
	Service    string                 `json:"service"`
	Components map[string]interface{} `json:"components"`
}

type HealthHandler struct {
	cache      services.Cache
	llmService services.LLMService
	logger     *slog.Logger
}

func NewHealthHandler(cache services.Cache, llmService services.LLMService, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		cache:      cache,
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

	// Test cache connection
	if err := h.cache.Ping(ctx); err != nil {
		h.logger.Warn("Cache health check failed", "error", err)
		components["cache"] = "unhealthy"
		overallStatus = "degraded"
	} else {
		components["cache"] = "healthy"
	}

	response := HealthResponse{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Service:    "roleplay-agent",
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
