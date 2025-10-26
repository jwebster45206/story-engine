package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// EventType represents the type of event being broadcast
type EventType string

const (
	EventTypeRequestQueued     EventType = "request.queued"
	EventTypeRequestProcessing EventType = "request.processing"
	EventTypeRequestCompleted  EventType = "request.completed"
	EventTypeRequestFailed     EventType = "request.failed"
	EventTypeChatChunk         EventType = "chat.chunk"
	EventTypeGameStateUpdated  EventType = "game.state_updated"
)

// Event represents a generic event structure
type Event struct {
	Type      EventType              `json:"type"`
	RequestID string                 `json:"request_id,omitempty"`
	GameID    string                 `json:"game_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Broadcaster publishes events to Redis Pub/Sub for SSE distribution
type Broadcaster struct {
	redisClient *redis.Client
	logger      *slog.Logger
}

// NewBroadcaster creates a new event broadcaster
func NewBroadcaster(redisClient *redis.Client, logger *slog.Logger) *Broadcaster {
	return &Broadcaster{
		redisClient: redisClient,
		logger:      logger,
	}
}

// PublishRequestQueued publishes a request.queued event
func (b *Broadcaster) PublishRequestQueued(ctx context.Context, gameID uuid.UUID, requestID string, requestType string) error {
	event := Event{
		Type:      EventTypeRequestQueued,
		RequestID: requestID,
		GameID:    gameID.String(),
		Data: map[string]interface{}{
			"status": "queued",
			"type":   requestType,
		},
	}
	return b.publishToGame(ctx, gameID, event)
}

// PublishRequestProcessing publishes a request.processing event
func (b *Broadcaster) PublishRequestProcessing(ctx context.Context, gameID uuid.UUID, requestID string, requestType string, userMessage string) error {
	event := Event{
		Type:      EventTypeRequestProcessing,
		RequestID: requestID,
		GameID:    gameID.String(),
		Data: map[string]interface{}{
			"status":       "processing",
			"type":         requestType,
			"user_message": userMessage,
		},
	}
	return b.publishToGame(ctx, gameID, event)
}

// PublishRequestCompleted publishes a request.completed event
func (b *Broadcaster) PublishRequestCompleted(ctx context.Context, gameID uuid.UUID, requestID string, result map[string]interface{}) error {
	event := Event{
		Type:      EventTypeRequestCompleted,
		RequestID: requestID,
		GameID:    gameID.String(),
		Data: map[string]interface{}{
			"status": "completed",
			"result": result,
		},
	}
	return b.publishToGame(ctx, gameID, event)
}

// PublishRequestFailed publishes a request.failed event
func (b *Broadcaster) PublishRequestFailed(ctx context.Context, gameID uuid.UUID, requestID string, errorMsg string) error {
	event := Event{
		Type:      EventTypeRequestFailed,
		RequestID: requestID,
		GameID:    gameID.String(),
		Data: map[string]interface{}{
			"status": "failed",
			"error":  errorMsg,
		},
	}
	return b.publishToGame(ctx, gameID, event)
}

// PublishChatChunk publishes a chat.chunk event (for streaming LLM responses)
func (b *Broadcaster) PublishChatChunk(ctx context.Context, gameID uuid.UUID, requestID string, content string, done bool) error {
	event := Event{
		Type:      EventTypeChatChunk,
		RequestID: requestID,
		GameID:    gameID.String(),
		Data: map[string]interface{}{
			"content": content,
			"done":    done,
		},
	}
	return b.publishToGame(ctx, gameID, event)
}

// PublishGameStateUpdated publishes a game.state_updated event
func (b *Broadcaster) PublishGameStateUpdated(ctx context.Context, gameID uuid.UUID, turn int, location string) error {
	event := Event{
		Type:   EventTypeGameStateUpdated,
		GameID: gameID.String(),
		Data: map[string]interface{}{
			"turn":     turn,
			"location": location,
		},
	}
	return b.publishToGame(ctx, gameID, event)
}

// publishToGame publishes an event to the game-specific channel
func (b *Broadcaster) publishToGame(ctx context.Context, gameID uuid.UUID, event Event) error {
	channel := fmt.Sprintf("game-events:%s", gameID.String())

	data, err := json.Marshal(event)
	if err != nil {
		b.logger.Error("Failed to marshal event", "error", err, "event", event)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := b.redisClient.Publish(ctx, channel, data).Err(); err != nil {
		b.logger.Error("Failed to publish event", "error", err, "channel", channel)
		return fmt.Errorf("failed to publish event: %w", err)
	}

	b.logger.Debug("Event published",
		"channel", channel,
		"event_type", event.Type,
		"request_id", event.RequestID,
	)

	return nil
}
