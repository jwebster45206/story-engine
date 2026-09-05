package state

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// MonsterStorage is the interface for loading monster templates
type MonsterStorage interface {
	GetMonster(ctx context.Context, templateID string) (*actor.Monster, error)
}

// DeltaWorker encapsulates the logic for applying deltas to game state,
// including variable updates and conditional overrides
type DeltaWorker struct {
	gs       *GameState
	delta    *conditionals.GameStateDelta
	scenario *scenario.Scenario
	logger   *slog.Logger
	queue    ChatQueue
	storage  MonsterStorage
	ctx      context.Context
}

// NewDeltaWorker creates a new delta worker for applying state changes
func NewDeltaWorker(gs *GameState, delta *conditionals.GameStateDelta, scen *scenario.Scenario, logger *slog.Logger) *DeltaWorker {
	return &DeltaWorker{
		gs:       gs,
		delta:    delta,
		scenario: scen,
		logger:   logger,
		ctx:      context.Background(),
	}
}

// WithQueue sets the queue service for enqueuing story events
// Returns the DeltaWorker for method chaining
func (dw *DeltaWorker) WithQueue(queue ChatQueue) *DeltaWorker {
	dw.queue = queue
	return dw
}

// WithStorage sets the storage service
// Returns the DeltaWorker for method chaining
func (dw *DeltaWorker) WithStorage(storage MonsterStorage) *DeltaWorker {
	dw.storage = storage
	return dw
}

// WithContext sets the context for queue operations
// Returns the DeltaWorker for method chaining
func (dw *DeltaWorker) WithContext(ctx context.Context) *DeltaWorker {
	dw.ctx = ctx
	return dw
}

// ApplyVars applies variable updates from the delta to the game state with snake_case conversion
func (dw *DeltaWorker) ApplyVars() {
	if dw.delta == nil {
		return
	}

	for k, v := range dw.delta.SetVars {
		snake := toSnakeCase(strings.ToLower(k))
		if dw.gs.Vars == nil {
			dw.gs.Vars = make(map[string]string)
		}
		dw.gs.Vars[snake] = v
	}
}

// Apply applies the delta to the game state (scene changes, items, location, game end)
func (dw *DeltaWorker) Apply() error {
	if dw.delta == nil {
		// No delta this turn - location cannot have changed.
		if dw.gs != nil {
			dw.gs.JustEntered = false
		}
		return nil
	}

	// Capture the pre-Apply location so we can flag JustEntered if it changes.
	// Scene loads also reset Location to the scene's opening; that case is
	// also a legitimate "just entered" signal.
	priorLocation := dw.gs.Location

	// Handle scene change
	if dw.delta.SceneChange != nil && dw.delta.SceneChange.To != "" &&
		// TODO: Add scene key/name disambiguation similar to locations
		// Scenes should have snake_case keys (e.g., "shipwright") and display names (e.g., "The Shipwright")
		// Use GetScene(keyOrName) helper to resolve both formats
		dw.delta.SceneChange.To != dw.gs.SceneName && dw.scenario.HasScene(dw.delta.SceneChange.To) {
		err := dw.gs.LoadScene(dw.scenario, dw.delta.SceneChange.To)
		if err != nil {
			return fmt.Errorf("failed to load scene: %w", err)
		}
		dw.gs.SceneName = dw.delta.SceneChange.To
	}

	// Handle location change
	if dw.delta.UserLocation != "" {
		locationKey := strings.ToLower(strings.TrimSpace(dw.delta.UserLocation))

		// Check if location exists in current game world
		if _, found := dw.gs.WorldLocations[locationKey]; found {
			// Update to the location key (ID), not the display name
			if dw.gs.Location != locationKey {
				if dw.logger != nil {
					dw.logger.Info("Location changed",
						"from", dw.gs.Location,
						"to", locationKey,
						"input", dw.delta.UserLocation)
				}
			}
			dw.gs.Location = locationKey
		} else {
			// Try matching by location name
			found := false
			for key, loc := range dw.gs.WorldLocations {
				if strings.ToLower(loc.Name) == locationKey {
					if dw.gs.Location != key {
						if dw.logger != nil {
							dw.logger.Info("Location changed",
								"from", dw.gs.Location,
								"to", key,
								"input", dw.delta.UserLocation)
						}
					}
					dw.gs.Location = key
					found = true
					break
				}
			}

			if !found {
				dw.logger.Warn("Could not find location",
					"input", dw.delta.UserLocation,
					"current", dw.gs.Location)
			}
		}
	}

	// Handle item events
	// TODO: Add item key/name disambiguation for all item operations
	// Items should have snake_case keys (e.g., "skeleton_key") and display names (e.g., "Skeleton Key")
	// Affects: AcquireItem, DropItem, GiveItem, MoveItem, UseItem
	// Consider adding GetItem(keyOrName) helper to resolve both formats
	for _, itemEvent := range dw.delta.ItemEvents {
		switch itemEvent.Action {
		case "acquire":
			dw.handleAcquireItem(itemEvent)
		case "drop":
			dw.handleDropItem(itemEvent)
		case "give":
			dw.handleGiveItem(itemEvent)
		case "move":
			dw.handleMoveItem(itemEvent)
		case "use":
			dw.handleUseItem(itemEvent)
		}
	}

	// Handle NPC events
	for _, npcEvent := range dw.delta.NPCEvents {
		dw.handleNPCEvent(npcEvent)
	}

	// Handle Monster events
	for _, monsterEvent := range dw.delta.MonsterEvents {
		dw.handleMonsterEvent(monsterEvent)
	}

	// TODO: Evaluate monster defeats (auto-despawn defeated monsters)
	// This runs after all delta operations to catch any HP changes
	// dw.gs.EvaluateDefeats()

	// Handle Game End
	if dw.delta.GameEnded != nil && *dw.delta.GameEnded {
		dw.gs.IsEnded = true
	}

	// Ensure that items are singletons
	dw.gs.NormalizeItems()

	// Sync locations for NPCs that are following other actors
	// This MUST be last to ensure we sync to final locations after all other changes
	dw.syncFollowingNPCs()

	// Flag JustEntered so the narrator knows on the next prompt build that
	// the player has just arrived. This self-resets on the following Apply()
	// when no location change occurs.
	dw.gs.JustEntered = dw.gs.Location != priorLocation

	return nil
}

// toSnakeCase converts a string to lower snake_case
func toSnakeCase(s string) string {
	var out strings.Builder
	prevUnderscore := false
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			r = r + ('a' - 'A')
		}
		if r == ' ' || r == '-' || r == '.' {
			if !prevUnderscore && i > 0 {
				out.WriteRune('_')
				prevUnderscore = true
			}
			continue
		}
		if r == '_' {
			if !prevUnderscore && i > 0 {
				out.WriteRune('_')
				prevUnderscore = true
			}
			continue
		}
		out.WriteRune(r)
		prevUnderscore = false
	}
	return out.String()
}
