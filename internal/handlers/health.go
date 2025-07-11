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
	Status     string            `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Service    string            `json:"service"`
	Components map[string]string `json:"components"`
}

type HealthHandler struct {
	redisService *services.RedisService
	logger       *slog.Logger
}

func NewHealthHandler(redisService *services.RedisService, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		redisService: redisService,
		logger:       logger,
	}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Debug("Health check requested",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr)

	// Check Redis health
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	components := make(map[string]string)
	overallStatus := "healthy"

	// Test Redis connection
	if err := h.redisService.Ping(ctx); err != nil {
		h.logger.Warn("Redis health check failed", "error", err)
		components["redis"] = "unhealthy"
		overallStatus = "degraded"
	} else {
		components["redis"] = "healthy"
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
