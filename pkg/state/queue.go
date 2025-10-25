package state

import (
	"context"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/queue"
)

// ChatQueue defines the interface for managing chat messages and story events
type ChatQueue interface {
	// GetFormattedEvents returns all queued chat messages and story events formatted as a single prompt
	GetFormattedEvents(ctx context.Context, gameID uuid.UUID) (string, error)

	// Clear removes all chat messages and story events for a game
	Clear(ctx context.Context, gameID uuid.UUID) error

	// EnqueueRequest adds a unified request to the global requests queue
	EnqueueRequest(ctx context.Context, req *queue.Request) error
}
