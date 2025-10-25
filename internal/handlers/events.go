package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services/events"
)

// EventsHandler handles Server-Sent Events (SSE) for real-time game updates
type EventsHandler struct {
	redisClient *redis.Client
	logger      *slog.Logger
}

// NewEventsHandler creates a new events handler
func NewEventsHandler(redisClient *redis.Client, logger *slog.Logger) *EventsHandler {
	return &EventsHandler{
		redisClient: redisClient,
		logger:      logger,
	}
}

// ServeHTTP handles SSE requests for game events
// GET /v1/events/games/{gameStateID}
func (h *EventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.logger.Warn("Method not allowed for events endpoint",
			"method", r.Method,
			"path", r.URL.Path)
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Method not allowed. Only GET is supported.",
		}); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	// Extract gameStateID from path
	// Expected: /v1/events/gamestate/{gameStateID}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) != 4 || pathParts[0] != "v1" || pathParts[1] != "events" || pathParts[2] != "gamestate" {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Invalid path. Expected /v1/events/gamestate/{gameStateID}",
		}); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	gameStateIDStr := pathParts[3]
	gameStateID, err := uuid.Parse(gameStateIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Error: "Invalid game state ID format.",
		}); err != nil {
			h.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	h.logger.Info("SSE connection established",
		"game_state_id", gameStateID.String(),
		"remote_addr", r.RemoteAddr)

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush headers immediately
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Subscribe to game events channel
	channel := fmt.Sprintf("game-events:%s", gameStateID.String())
	pubsub := h.redisClient.Subscribe(r.Context(), channel)
	defer func() {
		if err := pubsub.Close(); err != nil {
			h.logger.Error("Failed to close pubsub", "error", err)
		}
	}()

	h.logger.Debug("Subscribed to channel", "channel", channel)

	// Create message channel
	msgChan := pubsub.Channel()

	// Keepalive ticker (30 seconds)
	keepaliveTicker := time.NewTicker(30 * time.Second)
	defer keepaliveTicker.Stop()

	// Send initial connection event
	h.sendSSE(w, "connected", map[string]interface{}{
		"game_id": gameStateID.String(),
		"message": "Connected to event stream",
	})

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			h.logger.Info("SSE client disconnected",
				"game_state_id", gameStateID.String())
			return

		case msg := <-msgChan:
			// Received event from Redis
			var event events.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				h.logger.Error("Failed to unmarshal event", "error", err, "payload", msg.Payload)
				continue
			}

			// Forward event to client
			h.sendSSE(w, string(event.Type), event.Data)

		case <-keepaliveTicker.C:
			// Send keepalive comment
			if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
				h.logger.Error("Failed to write keepalive", "error", err)
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// sendSSE sends a Server-Sent Event to the client
func (h *EventsHandler) sendSSE(w http.ResponseWriter, eventType string, data interface{}) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.logger.Error("Failed to marshal SSE data", "error", err)
		return
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", eventType); err != nil {
		h.logger.Error("Failed to write event type", "error", err)
		return
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", string(dataJSON)); err != nil {
		h.logger.Error("Failed to write event data", "error", err)
		return
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
