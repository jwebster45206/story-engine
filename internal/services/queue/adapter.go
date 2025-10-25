package queue

import (
	"context"
	"log/slog"
)

// StoryEventQueueAdapter adapts StoryEventQueue for use by chat handlers
// Provides methods for getting formatted events and clearing queues
type StoryEventQueueAdapter struct {
	queue  *StoryEventQueue
	logger *slog.Logger
}

// NewStoryEventQueueAdapter creates a new adapter
func NewStoryEventQueueAdapter(queue *StoryEventQueue, logger *slog.Logger) *StoryEventQueueAdapter {
	return &StoryEventQueueAdapter{
		queue:  queue,
		logger: logger,
	}
}

// GetFormattedEvents returns all queued story events formatted as a single prompt
// This is the adapter method for chat handlers
func (a *StoryEventQueueAdapter) GetFormattedEvents(ctx context.Context, gameID string) (string, error) {
	return a.queue.GetFormattedEvents(ctx, gameID)
}

// Clear removes all story events for a game
func (a *StoryEventQueueAdapter) Clear(ctx context.Context, gameID string) error {
	return a.queue.Clear(ctx, gameID)
}

// Enqueue implements the state.StoryEventQueue interface for DeltaWorker
func (a *StoryEventQueueAdapter) Enqueue(ctx context.Context, gameID, eventPrompt string) error {
	return a.queue.Enqueue(ctx, gameID, eventPrompt)
}
