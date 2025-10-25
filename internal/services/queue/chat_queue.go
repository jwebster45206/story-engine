package queue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// ChatQueue manages story event queues per game
type ChatQueue struct {
	client *Client
	logger *slog.Logger
}

// NewChatQueue creates a new story event queue service
func NewChatQueue(client *Client, logger *slog.Logger) *ChatQueue {
	return &ChatQueue{
		client: client,
		logger: logger,
	}
}

// queueKey returns the Redis key for a game's story event queue
func (seq *ChatQueue) queueKey(gameStateID uuid.UUID) string {
	return fmt.Sprintf("story-events:%s", gameStateID.String())
}

// Enqueue adds a story event prompt to the end of the queue for a game
func (seq *ChatQueue) Enqueue(ctx context.Context, gameStateID uuid.UUID, eventPrompt string) error {
	key := seq.queueKey(gameStateID)

	err := seq.client.rdb.RPush(ctx, key, eventPrompt).Err()
	if err != nil {
		seq.logger.Error("Failed to enqueue story event",
			"error", err,
			"game_id", gameStateID,
			"key", key)
		return fmt.Errorf("failed to enqueue story event: %w", err)
	}

	seq.logger.Debug("Enqueued story event",
		"game_id", gameStateID,
		"prompt_preview", truncate(eventPrompt, 50))

	return nil
}

// Dequeue removes and returns all queued chat messages and story events for a game
func (seq *ChatQueue) Dequeue(ctx context.Context, gameStateID uuid.UUID) ([]string, error) {
	key := seq.queueKey(gameStateID)

	// Get all events
	events, err := seq.client.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil && err != redis.Nil {
		seq.logger.Error("Failed to dequeue story events",
			"error", err,
			"game_id", gameStateID,
			"key", key)
		return nil, fmt.Errorf("failed to dequeue story events: %w", err)
	}

	// Delete the queue
	if len(events) > 0 {
		if err := seq.client.rdb.Del(ctx, key).Err(); err != nil {
			seq.logger.Error("Failed to delete story event queue",
				"error", err,
				"game_id", gameStateID,
				"key", key)
			return nil, fmt.Errorf("failed to delete story event queue: %w", err)
		}

		seq.logger.Debug("Dequeued story events",
			"game_id", gameStateID,
			"count", len(events))
	}

	return events, nil
}

// Peek returns all story events without removing them
func (seq *ChatQueue) Peek(ctx context.Context, gameStateID uuid.UUID, limit int) ([]string, error) {
	key := seq.queueKey(gameStateID)

	end := int64(limit - 1)
	if limit <= 0 {
		end = -1 // Get all
	}

	events, err := seq.client.rdb.LRange(ctx, key, 0, end).Result()
	if err != nil && err != redis.Nil {
		seq.logger.Error("Failed to peek story events",
			"error", err,
			"game_id", gameStateID,
			"key", key)
		return nil, fmt.Errorf("failed to peek story events: %w", err)
	}

	return events, nil
}

// Clear removes all story events for a game
func (seq *ChatQueue) Clear(ctx context.Context, gameStateID uuid.UUID) error {
	key := seq.queueKey(gameStateID)

	err := seq.client.rdb.Del(ctx, key).Err()
	if err != nil {
		seq.logger.Error("Failed to clear story event queue",
			"error", err,
			"game_id", gameStateID,
			"key", key)
		return fmt.Errorf("failed to clear story event queue: %w", err)
	}

	seq.logger.Debug("Cleared story event queue", "game_id", gameStateID)
	return nil
}

// Depth returns the number of story events queued for a game
func (seq *ChatQueue) Depth(ctx context.Context, gameStateID uuid.UUID) (int, error) {
	key := seq.queueKey(gameStateID)

	count, err := seq.client.rdb.LLen(ctx, key).Result()
	if err != nil {
		seq.logger.Error("Failed to get story event queue depth",
			"error", err,
			"game_id", gameStateID,
			"key", key)
		return 0, fmt.Errorf("failed to get queue depth: %w", err)
	}

	return int(count), nil
}

// GetFormattedEvents returns all queued story events formatted as a single prompt
// This matches the behavior of GameState.GetStoryEvents()
func (seq *ChatQueue) GetFormattedEvents(ctx context.Context, gameStateID uuid.UUID) (string, error) {
	events, err := seq.Peek(ctx, gameStateID, 0)
	if err != nil {
		return "", err
	}

	if len(events) == 0 {
		return "", nil
	}

	// Format events similar to GameState.GetStoryEvents()
	var formatted string
	for i, event := range events {
		if i == 0 {
			formatted = "STORY EVENT: " + event
		} else {
			formatted += "\n\nSTORY EVENT: " + event
		}
	}

	return formatted, nil
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
