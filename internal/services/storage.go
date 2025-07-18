package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// HealthChecker defines basic health check capabilities
type HealthChecker interface {
	// Ping tests the service connection
	Ping(ctx context.Context) error
}

// Closer defines cleanup capabilities
type Closer interface {
	// Close closes the service connection
	Close() error
}

// Storage defines the interface for gamestate persistence
type Storage interface {
	HealthChecker
	Closer

	// SaveGameState saves a gamestate with the given UUID
	SaveGameState(ctx context.Context, uuid uuid.UUID, gamestate *state.GameState) error

	// LoadGameState retrieves a gamestate by UUID
	// Returns nil if the gamestate doesn't exist
	LoadGameState(ctx context.Context, uuid uuid.UUID) (*state.GameState, error)

	// DeleteGameState removes a gamestate by UUID
	DeleteGameState(ctx context.Context, uuid uuid.UUID) error

	// ListScenarios returns a map of scenario names to filenames
	ListScenarios(ctx context.Context) (map[string]string, error)

	// GetScenario retrieves a scenario by its filename
	GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error)
}
