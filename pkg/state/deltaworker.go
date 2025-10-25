package state

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// itemEvent is a type alias for the ItemEvents struct to avoid repetition
type itemEvent = struct {
	Item   string `json:"item"`
	Action string `json:"action"`
	From   *struct {
		Type string `json:"type"`
		Name string `json:"name,omitempty"`
	} `json:"from,omitempty"`
	To *struct {
		Type string `json:"type"`
		Name string `json:"name,omitempty"`
	} `json:"to,omitempty"`
	Consumed *bool `json:"consumed,omitempty"`
}

// DeltaWorker encapsulates the logic for applying deltas to game state,
// including variable updates and conditional overrides
type DeltaWorker struct {
	gs       *GameState
	delta    *GameStateDelta
	scenario *scenario.Scenario
	logger   *slog.Logger
	queue    StoryEventQueue // Optional queue service for story events
	ctx      context.Context
}

// NewDeltaWorker creates a new delta worker for applying state changes
func NewDeltaWorker(gs *GameState, delta *GameStateDelta, scen *scenario.Scenario, logger *slog.Logger) *DeltaWorker {
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
func (dw *DeltaWorker) WithQueue(queue StoryEventQueue) *DeltaWorker {
	dw.queue = queue
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

// ApplyConditionalOverrides evaluates conditionals and overrides delta fields based on results
// Returns a map of triggered conditional IDs to their conditionals for logging purposes
func (dw *DeltaWorker) ApplyConditionalOverrides() map[string]scenario.Conditional {
	if dw.scenario == nil {
		return nil
	}

	triggeredConditionals := dw.scenario.EvaluateConditionals(dw.gs)
	if len(triggeredConditionals) == 0 {
		return nil
	}

	// Process conditional actions and override delta
	for _, conditional := range triggeredConditionals {
		if conditional.Then.Scene != "" {
			dw.delta.SceneChange = &struct {
				To     string `json:"to"`
				Reason string `json:"reason"`
			}{
				To:     conditional.Then.Scene,
				Reason: "conditional",
			}
		}
		if conditional.Then.GameEnded != nil {
			dw.delta.GameEnded = conditional.Then.GameEnded
		}
	}

	return triggeredConditionals
}

// QueueStoryEvents evaluates story events and queues them for the next turn
// If a queue service is configured, events are enqueued to Redis.
// Otherwise, events are stored in gamestate (deprecated behavior for backwards compatibility).
// Returns the map of triggered story events for logging purposes
func (dw *DeltaWorker) QueueStoryEvents() map[string]scenario.StoryEvent {
	if dw.scenario == nil {
		return nil
	}

	triggeredEvents := dw.scenario.EvaluateStoryEvents(dw.gs)
	if len(triggeredEvents) == 0 {
		return nil
	}

	// If queue service is available, use it
	// Queue story events via Redis
	if dw.queue != nil {
		for _, event := range triggeredEvents {
			if err := dw.queue.Enqueue(dw.ctx, dw.gs.ID.String(), event.Prompt); err != nil {
				if dw.logger != nil {
					dw.logger.Error("Failed to enqueue story event to Redis",
						"error", err,
						"game_id", dw.gs.ID.String(),
						"event", event.Prompt)
				}
				// Don't fail the entire operation on queue errors
				// The event will be lost, but game state updates will proceed
			}
		}
	} else {
		// Queue service is required for story events
		if dw.logger != nil && len(triggeredEvents) > 0 {
			dw.logger.Error("No queue service configured, story events will be lost",
				"game_id", dw.gs.ID.String(),
				"event_count", len(triggeredEvents))
		}
	}

	return triggeredEvents
}

// Apply applies the delta to the game state (scene changes, items, location, game end)
func (dw *DeltaWorker) Apply() error {
	if dw.delta == nil {
		return nil
	}

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
		if locationKey, found := dw.scenario.GetLocation(dw.delta.UserLocation); found {
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
			dw.logger.Warn("Could not find location",
				"input", dw.delta.UserLocation,
				"current", dw.gs.Location)
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

	// Handle NPC movements
	for _, npcMovement := range dw.delta.NPCMovements {
		dw.handleNPCMovement(npcMovement)
	}

	// Handle Game End
	if dw.delta.GameEnded != nil && *dw.delta.GameEnded {
		dw.gs.IsEnded = true
	}

	// Ensure that items are singletons
	dw.gs.NormalizeItems()

	return nil
}

// handleAcquireItem adds an item to player inventory
func (dw *DeltaWorker) handleAcquireItem(itemEvent itemEvent) {
	itemExists := false
	for _, invItem := range dw.gs.Inventory {
		if invItem == itemEvent.Item {
			itemExists = true
			break
		}
	}
	if !itemExists {
		if dw.gs.Inventory == nil {
			dw.gs.Inventory = make([]string, 0)
		}
		dw.gs.Inventory = append(dw.gs.Inventory, itemEvent.Item)
	}
	// Remove from source if specified and not consumed
	if itemEvent.From != nil && (itemEvent.Consumed == nil || !*itemEvent.Consumed) {
		dw.removeItemFromSource(itemEvent.Item, itemEvent.From)
	}
}

// handleDropItem removes an item from player inventory
func (dw *DeltaWorker) handleDropItem(itemEvent itemEvent) {
	for i, invItem := range dw.gs.Inventory {
		if invItem == itemEvent.Item {
			dw.gs.Inventory = append(dw.gs.Inventory[:i], dw.gs.Inventory[i+1:]...)
			break
		}
	}
	// Add to destination if specified
	if itemEvent.To != nil {
		dw.addItemToDestination(itemEvent.Item, itemEvent.To)
	}
}

// handleGiveItem transfers an item between entities
func (dw *DeltaWorker) handleGiveItem(itemEvent itemEvent) {
	// Remove from source
	if itemEvent.From != nil {
		dw.removeItemFromSource(itemEvent.Item, itemEvent.From)
	} else {
		// Default to removing from player inventory if no source specified
		for i, invItem := range dw.gs.Inventory {
			if invItem == itemEvent.Item {
				dw.gs.Inventory = append(dw.gs.Inventory[:i], dw.gs.Inventory[i+1:]...)
				break
			}
		}
	}
	// Add to destination
	if itemEvent.To != nil {
		dw.addItemToDestination(itemEvent.Item, itemEvent.To)
	}
}

// handleMoveItem moves an item from one location/entity to another
func (dw *DeltaWorker) handleMoveItem(itemEvent itemEvent) {
	// Remove from source
	if itemEvent.From != nil {
		dw.removeItemFromSource(itemEvent.Item, itemEvent.From)
	}
	// Add to destination
	if itemEvent.To != nil {
		dw.addItemToDestination(itemEvent.Item, itemEvent.To)
	}
}

// handleUseItem uses an item and potentially consumes it
func (dw *DeltaWorker) handleUseItem(itemEvent itemEvent) {
	// If item is consumed, remove it from source
	if itemEvent.Consumed != nil && *itemEvent.Consumed {
		if itemEvent.From != nil {
			dw.removeItemFromSource(itemEvent.Item, itemEvent.From)
		} else {
			// Default to removing from player inventory if no source specified
			for i, invItem := range dw.gs.Inventory {
				if invItem == itemEvent.Item {
					dw.gs.Inventory = append(dw.gs.Inventory[:i], dw.gs.Inventory[i+1:]...)
					break
				}
			}
		}
	}
}

// handleNPCMovement updates an NPC's location
func (dw *DeltaWorker) handleNPCMovement(movement NPCMovement) {
	// Try to find the NPC by ID or name
	npcKey, found := dw.scenario.GetNPC(movement.NPCID)
	if !found {
		if dw.logger != nil {
			dw.logger.Warn("NPC not found for movement",
				"npc_id", movement.NPCID,
				"to_location", movement.ToLocation)
		}
		return
	}

	// Verify the destination location exists
	locationKey, found := dw.scenario.GetLocation(movement.ToLocation)
	if !found {
		if dw.logger != nil {
			dw.logger.Warn("Location not found for NPC movement",
				"npc_id", movement.NPCID,
				"to_location", movement.ToLocation)
		}
		return
	}

	// Update the NPC's location in game state
	if npc, exists := dw.gs.NPCs[npcKey]; exists {
		oldLocation := npc.Location
		npc.Location = locationKey
		dw.gs.NPCs[npcKey] = npc

		if dw.logger != nil {
			dw.logger.Info("NPC moved",
				"npc", npcKey,
				"from", oldLocation,
				"to", locationKey)
		}
	} else {
		if dw.logger != nil {
			dw.logger.Warn("NPC not found in game state",
				"npc_key", npcKey)
		}
	}
}

// removeItemFromSource removes an item from the specified source
func (dw *DeltaWorker) removeItemFromSource(item string, from *struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}) {
	gs := dw.gs
	switch from.Type {
	case "player":
		// Remove from player inventory
		for i, invItem := range gs.Inventory {
			if invItem == item {
				gs.Inventory = append(gs.Inventory[:i], gs.Inventory[i+1:]...)
				break
			}
		}
	case "location":
		// Remove from location
		for key, loc := range gs.WorldLocations {
			if loc.Name == from.Name {
				for i, invItem := range loc.Items {
					if invItem == item {
						loc.Items = append(loc.Items[:i], loc.Items[i+1:]...)
						gs.WorldLocations[key] = loc // Write back
						break
					}
				}
				break
			}
		}
	case "npc":
		// Remove from NPC
		npcKey, found := dw.scenario.GetNPC(from.Name)
		if !found {
			return
		}
		if npc, ok := gs.NPCs[npcKey]; ok {
			for i, invItem := range npc.Items {
				if invItem == item {
					npc.Items = append(npc.Items[:i], npc.Items[i+1:]...)
					gs.NPCs[npcKey] = npc // Write back
					break
				}
			}
		}
	}
}

// addItemToDestination adds an item to the specified destination
func (dw *DeltaWorker) addItemToDestination(item string, to *struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}) {
	gs := dw.gs
	switch to.Type {
	case "player":
		// Add to player inventory (check for duplicates)
		itemExists := slices.Contains(gs.Inventory, item)
		if !itemExists {
			if gs.Inventory == nil {
				gs.Inventory = make([]string, 0)
			}
			gs.Inventory = append(gs.Inventory, item)
		}
	case "location":
		// Add to location
		for key, loc := range gs.WorldLocations {
			if loc.Name == to.Name {
				if loc.Items == nil {
					loc.Items = make([]string, 0)
				}
				loc.Items = append(loc.Items, item)
				gs.WorldLocations[key] = loc // Write back
				break
			}
		}
	case "npc":
		// Add to NPC
		npcKey, found := dw.scenario.GetNPC(to.Name)
		if !found {
			return
		}
		if npc, ok := gs.NPCs[npcKey]; ok {
			if npc.Items == nil {
				npc.Items = make([]string, 0)
			}
			npc.Items = append(npc.Items, item)
			gs.NPCs[npcKey] = npc // Write back
		}
	}
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
