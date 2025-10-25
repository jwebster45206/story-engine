package state

import "context"

// StoryEventQueue defines the interface for managing story events
// This allows handlers and DeltaWorker to remain testable without a hard dependency on Redis
type StoryEventQueue interface {
	// Enqueue adds a story event prompt to the queue for a game
	Enqueue(ctx context.Context, gameID, eventPrompt string) error

	// GetFormattedEvents returns all queued story events formatted as a single prompt
	GetFormattedEvents(ctx context.Context, gameID string) (string, error)

	// Clear removes all story events for a game
	Clear(ctx context.Context, gameID string) error
}
