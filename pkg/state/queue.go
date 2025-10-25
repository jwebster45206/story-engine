package state

import (
	"context"

	"github.com/google/uuid"
)

// StoryEventQueue defines the interface for managing story events
type StoryEventQueue interface {
	// Enqueue adds a story event prompt to the queue for a game
	Enqueue(ctx context.Context, gameID uuid.UUID, eventPrompt string) error

	// GetFormattedEvents returns all queued story events formatted as a single prompt
	GetFormattedEvents(ctx context.Context, gameID uuid.UUID) (string, error)

	// Clear removes all story events for a game
	Clear(ctx context.Context, gameID uuid.UUID) error
}
