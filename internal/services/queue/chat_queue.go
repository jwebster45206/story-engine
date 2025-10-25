package queue

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/queue"
)

// ChatQueue manages a queue of chat messages and story events for games
type ChatQueue struct {
	client *Client
}

func NewChatQueue(client *Client) *ChatQueue {
	return &ChatQueue{
		client: client,
	}
}

func queueKey(gameStateID uuid.UUID) string {
	return fmt.Sprintf("story-events:%s", gameStateID.String())
}

// Enqueue adds a story event prompt to the end of the queue for a game
func (seq *ChatQueue) Enqueue(ctx context.Context, gameStateID uuid.UUID, eventPrompt string) error {
	key := queueKey(gameStateID)
	err := seq.client.rdb.RPush(ctx, key, eventPrompt).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue story event: %w", err)
	}
	return nil
}

// Dequeue removes and returns all queued chat messages and story events for a game
func (seq *ChatQueue) Dequeue(ctx context.Context, gameStateID uuid.UUID) ([]string, error) {
	key := queueKey(gameStateID)

	// Get all events
	events, err := seq.client.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to dequeue story events: %w", err)
	}
	// Delete the queue
	if len(events) > 0 {
		if err := seq.client.rdb.Del(ctx, key).Err(); err != nil {
			return nil, fmt.Errorf("failed to clear story event queue after dequeue: %w", err)
		}
	}
	return events, nil
}

// Peek returns all story events without removing them
func (seq *ChatQueue) Peek(ctx context.Context, gameStateID uuid.UUID, limit int) ([]string, error) {
	key := queueKey(gameStateID)

	end := int64(limit - 1)
	if limit <= 0 {
		end = -1 // Get all
	}
	events, err := seq.client.rdb.LRange(ctx, key, 0, end).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to peek story events: %w", err)
	}
	return events, nil
}

// Clear removes all story events for a game
func (seq *ChatQueue) Clear(ctx context.Context, gameStateID uuid.UUID) error {
	key := queueKey(gameStateID)
	err := seq.client.rdb.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to clear story event queue: %w", err)
	}
	return nil
}

// Depth returns the number of story events queued for a game
func (seq *ChatQueue) Depth(ctx context.Context, gameStateID uuid.UUID) (int, error) {
	key := queueKey(gameStateID)
	count, err := seq.client.rdb.LLen(ctx, key).Result()
	if err != nil {
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

// EnqueueRequest adds a unified request to the global requests queue
func (seq *ChatQueue) EnqueueRequest(ctx context.Context, req *queue.Request) error {
	data, err := req.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize request: %w", err)
	}

	err = seq.client.rdb.RPush(ctx, "requests", data).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue request: %w", err)
	}
	return nil
}

// DequeueRequest removes and returns the next request from the global queue
// Returns nil if queue is empty
func (seq *ChatQueue) DequeueRequest(ctx context.Context) (*queue.Request, error) {
	result, err := seq.client.rdb.LPop(ctx, "requests").Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Queue is empty
		}
		return nil, fmt.Errorf("failed to dequeue request: %w", err)
	}

	req, err := queue.FromJSON([]byte(result))
	if err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	return req, nil
}

// BlockingDequeueRequest blocks until a request is available, then returns it
// timeout is in seconds, 0 means wait forever
func (seq *ChatQueue) BlockingDequeueRequest(ctx context.Context, timeout int) (*queue.Request, error) {
	result, err := seq.client.rdb.BLPop(ctx, 0, "requests").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue request: %w", err)
	}

	// BLPop returns [key, value]
	if len(result) != 2 {
		return nil, fmt.Errorf("unexpected BLPop result: %v", result)
	}

	req, err := queue.FromJSON([]byte(result[1]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	return req, nil
}

// RequestQueueDepth returns the number of requests in the global queue
func (seq *ChatQueue) RequestQueueDepth(ctx context.Context) (int, error) {
	count, err := seq.client.rdb.LLen(ctx, "requests").Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get request queue depth: %w", err)
	}
	return int(count), nil
}
