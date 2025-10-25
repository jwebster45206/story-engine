package state

import (
	"context"

	"github.com/google/uuid"
)

// ChatQueue defines the interface for managing chat messages and story events
type ChatQueue interface {
	// Enqueue adds a chat message or story event prompt to the queue for a game
	Enqueue(ctx context.Context, gameID uuid.UUID, eventPrompt string) error

	// GetFormattedEvents returns all queued chat messages and story events formatted as a single prompt
	GetFormattedEvents(ctx context.Context, gameID uuid.UUID) (string, error)

	// Clear removes all chat messages and story events for a game
	Clear(ctx context.Context, gameID uuid.UUID) error
}
