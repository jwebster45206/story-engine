package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Service:   "roleplay-agent",
	}

	slog.Debug("Health check requested",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Error encoding health response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
