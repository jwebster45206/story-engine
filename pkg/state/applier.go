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

// Applier encapsulates the logic for applying deltas to game state,
// including variable updates and conditional overrides
type Applier struct {
	gs       *GameState
	delta    *conditionals.GameStateDelta
	scenario *scenario.Scenario
	logger   *slog.Logger
	queue    ChatQueue
	storage  MonsterStorage
	ctx      context.Context
}

// NewApplier creates an Applier for applying state changes
func NewApplier(gs *GameState, delta *conditionals.GameStateDelta, scen *scenario.Scenario, logger *slog.Logger) *Applier {
	return &Applier{
		gs:       gs,
		delta:    delta,
		scenario: scen,
		logger:   logger,
		ctx:      context.Background(),
	}
}

// WithQueue sets the queue service for enqueuing story events
// Returns the Applier for method chaining
func (a *Applier) WithQueue(queue ChatQueue) *Applier {
	a.queue = queue
	return a
}

// WithStorage sets the storage service
// Returns the Applier for method chaining
func (a *Applier) WithStorage(storage MonsterStorage) *Applier {
	a.storage = storage
	return a
}

// WithContext sets the context for queue operations
// Returns the Applier for method chaining
func (a *Applier) WithContext(ctx context.Context) *Applier {
	a.ctx = ctx
	return a
}

// ApplyVars applies variable updates from the delta to the game state with snake_case conversion
func (a *Applier) ApplyVars() {
	if a.delta == nil {
		return
	}

	for k, v := range a.delta.SetVars {
		snake := toSnakeCase(strings.ToLower(k))
		if a.gs.Vars == nil {
			a.gs.Vars = make(map[string]string)
		}
		a.gs.Vars[snake] = v
	}
}

// Apply applies the delta to the game state (scene changes, items, location, game end)
func (a *Applier) Apply() error {
	if a.delta == nil {
		// No delta this turn - location cannot have changed.
		if a.gs != nil {
			a.gs.JustEntered = false
		}
		return nil
	}

	// Capture the pre-Apply location so we can flag JustEntered if it changes.
	// Scene loads also reset Location to the scene's opening; that case is
	// also a legitimate "just entered" signal.
	priorLocation := a.gs.Location

	// Handle scene change
	if a.delta.SceneChange != nil && a.delta.SceneChange.To != "" &&
		// TODO: Add scene key/name disambiguation similar to locations
		// Scenes should have snake_case keys (e.g., "shipwright") and display names (e.g., "The Shipwright")
		// Use GetScene(keyOrName) helper to resolve both formats
		a.delta.SceneChange.To != a.gs.SceneName && a.scenario.HasScene(a.delta.SceneChange.To) {
		err := a.gs.LoadScene(a.scenario, a.delta.SceneChange.To)
		if err != nil {
			return fmt.Errorf("failed to load scene: %w", err)
		}
		a.gs.SceneName = a.delta.SceneChange.To
	}

	// Handle location change
	if a.delta.UserLocation != "" {
		locationKey := strings.ToLower(strings.TrimSpace(a.delta.UserLocation))

		// Check if location exists in current game world
		if _, found := a.gs.WorldLocations[locationKey]; found {
			// Update to the location key (ID), not the display name
			if a.gs.Location != locationKey {
				if a.logger != nil {
					a.logger.Info("Location changed",
						"from", a.gs.Location,
						"to", locationKey,
						"input", a.delta.UserLocation)
				}
			}
			a.gs.Location = locationKey
		} else {
			// Try matching by location name
			found := false
			for key, loc := range a.gs.WorldLocations {
				if strings.ToLower(loc.Name) == locationKey {
					if a.gs.Location != key {
						if a.logger != nil {
							a.logger.Info("Location changed",
								"from", a.gs.Location,
								"to", key,
								"input", a.delta.UserLocation)
						}
					}
					a.gs.Location = key
					found = true
					break
				}
			}

			if !found {
				a.logger.Warn("Could not find location",
					"input", a.delta.UserLocation,
					"current", a.gs.Location)
			}
		}
	}

	// Handle item events
	// TODO: Add item key/name disambiguation for all item operations
	// Items should have snake_case keys (e.g., "skeleton_key") and display names (e.g., "Skeleton Key")
	// Affects: AcquireItem, DropItem, GiveItem, MoveItem, UseItem
	// Consider adding GetItem(keyOrName) helper to resolve both formats
	for _, itemEvent := range a.delta.ItemEvents {
		switch itemEvent.Action {
		case "acquire":
			a.handleAcquireItem(itemEvent)
		case "drop":
			a.handleDropItem(itemEvent)
		case "give":
			a.handleGiveItem(itemEvent)
		case "move":
			a.handleMoveItem(itemEvent)
		case "use":
			a.handleUseItem(itemEvent)
		}
	}

	// Handle NPC events
	for _, npcEvent := range a.delta.NPCEvents {
		a.handleNPCEvent(npcEvent)
	}

	// Handle Monster events
	for _, monsterEvent := range a.delta.MonsterEvents {
		a.handleMonsterEvent(monsterEvent)
	}

	// TODO: Evaluate monster defeats (auto-despawn defeated monsters)
	// This runs after all delta operations to catch any HP changes
	// a.gs.EvaluateDefeats()

	// Handle Game End
	if a.delta.GameEnded != nil && *a.delta.GameEnded {
		a.gs.IsEnded = true
	}

	// Ensure that items are singletons
	a.gs.NormalizeItems()

	// Sync locations for NPCs that are following other actors
	// This MUST be last to ensure we sync to final locations after all other changes
	a.syncFollowingNPCs()

	// Flag JustEntered so the narrator knows on the next prompt build that
	// the player has just arrived. This self-resets on the following Apply()
	// when no location change occurs.
	a.gs.JustEntered = a.gs.Location != priorLocation

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
