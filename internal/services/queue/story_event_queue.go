package queue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-redis/redis/v8"
)

// StoryEventQueue manages story event queues per game
type StoryEventQueue struct {
	client *Client
	logger *slog.Logger
}

// NewStoryEventQueue creates a new story event queue service
func NewStoryEventQueue(client *Client, logger *slog.Logger) *StoryEventQueue {
	return &StoryEventQueue{
		client: client,
		logger: logger,
	}
}

// queueKey returns the Redis key for a game's story event queue
func (seq *StoryEventQueue) queueKey(gameID string) string {
	return fmt.Sprintf("story-events:%s", gameID)
}

// Enqueue adds a story event prompt to the end of the queue for a game
func (seq *StoryEventQueue) Enqueue(ctx context.Context, gameID, eventPrompt string) error {
	key := seq.queueKey(gameID)

	err := seq.client.rdb.RPush(ctx, key, eventPrompt).Err()
	if err != nil {
		seq.logger.Error("Failed to enqueue story event",
			"error", err,
			"game_id", gameID,
			"key", key)
		return fmt.Errorf("failed to enqueue story event: %w", err)
	}

	seq.logger.Debug("Enqueued story event",
		"game_id", gameID,
		"prompt_preview", truncate(eventPrompt, 50))

	return nil
}

// Dequeue removes and returns all story events for a game
func (seq *StoryEventQueue) Dequeue(ctx context.Context, gameID string) ([]string, error) {
	key := seq.queueKey(gameID)

	// Get all events
	events, err := seq.client.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil && err != redis.Nil {
		seq.logger.Error("Failed to dequeue story events",
			"error", err,
			"game_id", gameID,
			"key", key)
		return nil, fmt.Errorf("failed to dequeue story events: %w", err)
	}

	// Delete the queue
	if len(events) > 0 {
		if err := seq.client.rdb.Del(ctx, key).Err(); err != nil {
			seq.logger.Error("Failed to delete story event queue",
				"error", err,
				"game_id", gameID,
				"key", key)
			return nil, fmt.Errorf("failed to delete story event queue: %w", err)
		}

		seq.logger.Debug("Dequeued story events",
			"game_id", gameID,
			"count", len(events))
	}

	return events, nil
}

// Peek returns all story events without removing them
func (seq *StoryEventQueue) Peek(ctx context.Context, gameID string, limit int) ([]string, error) {
	key := seq.queueKey(gameID)

	end := int64(limit - 1)
	if limit <= 0 {
		end = -1 // Get all
	}

	events, err := seq.client.rdb.LRange(ctx, key, 0, end).Result()
	if err != nil && err != redis.Nil {
		seq.logger.Error("Failed to peek story events",
			"error", err,
			"game_id", gameID,
			"key", key)
		return nil, fmt.Errorf("failed to peek story events: %w", err)
	}

	return events, nil
}

// Clear removes all story events for a game
func (seq *StoryEventQueue) Clear(ctx context.Context, gameID string) error {
	key := seq.queueKey(gameID)

	err := seq.client.rdb.Del(ctx, key).Err()
	if err != nil {
		seq.logger.Error("Failed to clear story event queue",
			"error", err,
			"game_id", gameID,
			"key", key)
		return fmt.Errorf("failed to clear story event queue: %w", err)
	}

	seq.logger.Debug("Cleared story event queue", "game_id", gameID)
	return nil
}

// Depth returns the number of story events queued for a game
func (seq *StoryEventQueue) Depth(ctx context.Context, gameID string) (int, error) {
	key := seq.queueKey(gameID)

	count, err := seq.client.rdb.LLen(ctx, key).Result()
	if err != nil {
		seq.logger.Error("Failed to get story event queue depth",
			"error", err,
			"game_id", gameID,
			"key", key)
		return 0, fmt.Errorf("failed to get queue depth: %w", err)
	}

	return int(count), nil
}

// GetFormattedEvents returns all queued story events formatted as a single prompt
// This matches the behavior of GameState.GetStoryEvents()
func (seq *StoryEventQueue) GetFormattedEvents(ctx context.Context, gameID string) (string, error) {
	events, err := seq.Peek(ctx, gameID, 0)
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
