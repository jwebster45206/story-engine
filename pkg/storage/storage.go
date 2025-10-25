package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// Storage defines a unified interface for all storage operations
// This interface combines gamestate persistence (Redis) with resource loading (filesystem)
type Storage interface {
	// Health and lifecycle
	Ping(ctx context.Context) error
	Close() error

	// GameState operations (Redis-backed)
	SaveGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error
	LoadGameState(ctx context.Context, id uuid.UUID) (*state.GameState, error)
	DeleteGameState(ctx context.Context, id uuid.UUID) error

	// Scenario operations (filesystem-backed)
	ListScenarios(ctx context.Context) (map[string]string, error)
	GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error)

	// Narrator operations (filesystem-backed)
	GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error)
	ListNarrators(ctx context.Context) ([]string, error)

	// PC operations (filesystem-backed, returns PCSpec not PC)
	// GetPCSpec loads a PC spec from storage but does NOT construct the d20.Actor
	// Use actor.NewPCFromSpec to build the full PC from the returned spec
	GetPCSpec(ctx context.Context, pcID string) (*actor.PCSpec, error)
	ListPCs(ctx context.Context) ([]string, error)
}
