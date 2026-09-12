package storage

import (
	"context"
	"errors"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// ErrNotFound is returned when a stored resource does not exist.
var ErrNotFound = errors.New("not found")

// Storage defines a unified interface for all storage operations
// This interface combines gamestate persistence (Redis) with resource loading (filesystem)
type Storage interface {
	// Health and lifecycle
	Ping(ctx context.Context) error
	Close() error

	// GameState operations (Redis-backed)
	CreateGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error
	UpdateGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error
	LoadGameState(ctx context.Context, id uuid.UUID) (*state.GameState, error)
	DeleteGameState(ctx context.Context, id uuid.UUID) error
	GetOwner(ctx context.Context, id uuid.UUID) (ownerID uuid.UUID, err error)

	// Scenario operations (filesystem-backed)
	ListScenarios(ctx context.Context) (map[string]string, error)
	GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error)

	// Narrator operations (filesystem-backed)
	GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error)
	ListNarrators(ctx context.Context) ([]string, error)

	// PC operations (filesystem-backed)
	GetPC(ctx context.Context, pcID string) (*character.PC, error)
	ListPCs(ctx context.Context) ([]string, error)

	// Monster operations (filesystem-backed, returns Monster template)
	// Use character.NewMonster to create instances from the template
	GetMonster(ctx context.Context, templateID string) (*character.Monster, error)
	ListMonsters(ctx context.Context) (map[string]string, error) // map[name]templateID

	// NPC operations (filesystem-backed, returns NPC template from data/npcs/)
	// Use character.NewNPCFromTemplate to merge template with scenario overrides
	GetNPC(ctx context.Context, templateID string) (*character.NPC, error)
	ListNPCs(ctx context.Context) (map[string]string, error) // map[name]templateID
}
