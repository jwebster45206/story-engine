package state

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/queue"
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
	delta    *conditionals.GameStateDelta
	scenario *scenario.Scenario
	logger   *slog.Logger
	queue    ChatQueue
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
// Also handles prompt-based actions (story events, etc.) by queuing them
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
	for conditionalID, conditional := range triggeredConditionals {
		// Merge conditional.Then (GameStateDelta) into the existing delta
		dw.mergeDelta(&conditional.Then, conditionalID)
	}

	return triggeredConditionals
}

// mergeDelta merges a conditional's delta into the worker's delta, with special handling for prompts
func (dw *DeltaWorker) mergeDelta(conditionalDelta *conditionals.GameStateDelta, conditionalID string) {
	if conditionalDelta == nil {
		return
	}

	// Merge scene change
	if conditionalDelta.SceneChange != nil && conditionalDelta.SceneChange.To != "" {
		dw.delta.SceneChange = &struct {
			To     string `json:"to"`
			Reason string `json:"reason"`
		}{
			To:     conditionalDelta.SceneChange.To,
			Reason: "conditional",
		}
	}

	// Merge game ended state, overriding any previous value
	if conditionalDelta.GameEnded != nil {
		dw.delta.GameEnded = conditionalDelta.GameEnded
	}

	// Merge user location, overriding any previous value
	if conditionalDelta.UserLocation != "" {
		dw.delta.UserLocation = conditionalDelta.UserLocation
	}

	// Merge variables, overriding any previous values
	if len(conditionalDelta.SetVars) > 0 {
		if dw.delta.SetVars == nil {
			dw.delta.SetVars = make(map[string]string)
		}
		maps.Copy(dw.delta.SetVars, conditionalDelta.SetVars)
	}

	// Merge item events
	if len(conditionalDelta.ItemEvents) > 0 {
		dw.delta.ItemEvents = append(dw.delta.ItemEvents, conditionalDelta.ItemEvents...)
	}

	// Merge NPC events
	if len(conditionalDelta.NPCEvents) > 0 {
		dw.delta.NPCEvents = append(dw.delta.NPCEvents, conditionalDelta.NPCEvents...)
	}

	// Merge location events
	if len(conditionalDelta.LocationEvents) > 0 {
		dw.delta.LocationEvents = append(dw.delta.LocationEvents, conditionalDelta.LocationEvents...)
	}

	// Handle prompt - any prompt in a conditional is treated as a story event
	if conditionalDelta.Prompt != nil {
		prompt := *conditionalDelta.Prompt
		// Check if this story event has already fired
		if !dw.hasStoryEventFired(conditionalID) {
			// Queue the story event
			dw.queueStoryEvent(conditionalID, prompt)
		} else if dw.logger != nil {
			dw.logger.Debug("Story event already fired, skipping",
				"game_state_id", dw.gs.ID.String(),
				"conditional_id", conditionalID)
		}
	}
}

// hasStoryEventFired checks if a story event has already been fired
func (dw *DeltaWorker) hasStoryEventFired(conditionalID string) bool {
	if dw.gs == nil || dw.gs.FiredStoryEvents == nil {
		return false
	}
	for _, firedID := range dw.gs.FiredStoryEvents {
		if firedID == conditionalID {
			return true
		}
	}
	return false
}

// queueStoryEvent queues a single story event for the next turn and marks it as fired
func (dw *DeltaWorker) queueStoryEvent(conditionalID string, eventText string) {
	// Queue service is required for story events
	if dw.queue == nil {
		if dw.logger != nil {
			dw.logger.Error("No queue service configured, story event will be lost",
				"game_state_id", dw.gs.ID.String(),
				"event", eventText)
		}
		return
	}

	req := &queue.Request{
		RequestID:   uuid.New().String(),
		Type:        queue.RequestTypeStoryEvent,
		GameStateID: dw.gs.ID,
		EventPrompt: eventText,
		EnqueuedAt:  time.Now(),
	}

	if err := dw.queue.EnqueueRequest(dw.ctx, req); err != nil {
		if dw.logger != nil {
			dw.logger.Error("Failed to enqueue story event to unified queue",
				"error", err,
				"game_state_id", dw.gs.ID.String(),
				"request_id", req.RequestID,
				"event", eventText)
		}
	} else {
		// Successfully queued - mark this story event as fired
		if dw.gs.FiredStoryEvents == nil {
			dw.gs.FiredStoryEvents = make([]string, 0)
		}
		dw.gs.FiredStoryEvents = append(dw.gs.FiredStoryEvents, conditionalID)

		if dw.logger != nil {
			dw.logger.Info("Story event enqueued to unified queue",
				"game_state_id", dw.gs.ID.String(),
				"request_id", req.RequestID,
				"conditional_id", conditionalID,
				"event_prompt", eventText)
		}
	}
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

	// Handle location events
	for _, locationEvent := range dw.delta.LocationEvents {
		dw.handleLocationEvent(locationEvent)
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

// handleNPCEvent processes an NPC event
func (dw *DeltaWorker) handleNPCEvent(event conditionals.NPCEvent) {
	// Handle location change if present
	if event.LocationChange != nil {
		dw.handleNPCLocationChange(event.NPCID, event.LocationChange.To, event.LocationChange.Reason)
	}
}

// handleNPCLocationChange updates an NPC's location
func (dw *DeltaWorker) handleNPCLocationChange(npcID, toLocation, reason string) {
	// Normalize the NPC identifier
	npcKey := strings.ToLower(strings.TrimSpace(npcID))

	// Try to find the NPC in game state by key first
	npc, npcExists := dw.gs.NPCs[npcKey]
	if !npcExists {
		// Try matching by NPC name
		for key, n := range dw.gs.NPCs {
			if strings.ToLower(n.Name) == npcKey {
				npcKey = key
				npc = n
				npcExists = true
				break
			}
		}
	}

	if !npcExists {
		if dw.logger != nil {
			dw.logger.Warn("NPC not found for movement",
				"npc_id", npcID,
				"to_location", toLocation,
				"reason", reason)
		}
		return
	}

	// Normalize and verify the destination location exists
	locationKey := strings.ToLower(strings.TrimSpace(toLocation))
	_, locationExists := dw.gs.WorldLocations[locationKey]
	if !locationExists {
		// Try matching by location name
		for key, loc := range dw.gs.WorldLocations {
			if strings.ToLower(loc.Name) == locationKey {
				locationKey = key
				locationExists = true
				break
			}
		}
	}

	if !locationExists {
		if dw.logger != nil {
			dw.logger.Warn("Location not found for NPC movement",
				"npc_id", npcID,
				"to_location", toLocation,
				"reason", reason)
		}
		return
	}

	// Update the NPC's location in game state
	oldLocation := npc.Location
	npc.Location = locationKey
	dw.gs.NPCs[npcKey] = npc

	if dw.logger != nil {
		dw.logger.Info("NPC moved",
			"npc", npcKey,
			"from", oldLocation,
			"to", locationKey,
			"reason", reason)
	}
}

// handleLocationEvent processes a location event
func (dw *DeltaWorker) handleLocationEvent(event conditionals.LocationEvent) {
	// Normalize location ID
	locationKey := strings.ToLower(strings.TrimSpace(event.LocationID))

	// Try to find the location in game state by key first
	location, locationExists := dw.gs.WorldLocations[locationKey]
	if !locationExists {
		// Try matching by location name
		for key, loc := range dw.gs.WorldLocations {
			if strings.ToLower(loc.Name) == locationKey {
				locationKey = key
				location = loc
				locationExists = true
				break
			}
		}
	}

	if !locationExists {
		if dw.logger != nil {
			dw.logger.Warn("Location not found for location event",
				"location_id", event.LocationID)
		}
		return
	}

	// Process exit changes
	if len(event.ExitChanges) > 0 {
		// Initialize BlockedExits map if needed
		if location.BlockedExits == nil {
			location.BlockedExits = make(map[string]string)
		}

		for _, exitChange := range event.ExitChanges {
			exitID := strings.ToLower(strings.TrimSpace(exitChange.ExitID))

			switch exitChange.Status {
			case "blocked":
				// Add or update the blocked exit
				reason := exitChange.Reason
				if reason == "" {
					reason = "blocked"
				}
				location.BlockedExits[exitID] = reason

				if dw.logger != nil {
					dw.logger.Info("Exit blocked",
						"location", locationKey,
						"exit", exitID,
						"reason", reason)
				}

			case "unblocked":
				// Remove the exit from blocked list
				if _, wasBlocked := location.BlockedExits[exitID]; wasBlocked {
					delete(location.BlockedExits, exitID)

					if dw.logger != nil {
						dw.logger.Info("Exit unblocked",
							"location", locationKey,
							"exit", exitID)
					}
				}

			default:
				if dw.logger != nil {
					dw.logger.Warn("Unknown exit status",
						"location", locationKey,
						"exit", exitID,
						"status", exitChange.Status)
				}
			}
		}

		// Write back the modified location
		dw.gs.WorldLocations[locationKey] = location
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
		npcKey := strings.ToLower(strings.TrimSpace(from.Name))

		// Try to find NPC in game state by key first
		if npc, ok := gs.NPCs[npcKey]; ok {
			for i, invItem := range npc.Items {
				if invItem == item {
					npc.Items = append(npc.Items[:i], npc.Items[i+1:]...)
					gs.NPCs[npcKey] = npc // Write back
					break
				}
			}
			return
		}

		// Try matching by NPC name
		for key, npc := range gs.NPCs {
			if strings.ToLower(npc.Name) == npcKey {
				for i, invItem := range npc.Items {
					if invItem == item {
						npc.Items = append(npc.Items[:i], npc.Items[i+1:]...)
						gs.NPCs[key] = npc // Write back
						break
					}
				}
				break
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
		npcKey := strings.ToLower(strings.TrimSpace(to.Name))

		// Try to find NPC in game state by key first
		if npc, ok := gs.NPCs[npcKey]; ok {
			if npc.Items == nil {
				npc.Items = make([]string, 0)
			}
			npc.Items = append(npc.Items, item)
			gs.NPCs[npcKey] = npc // Write back
			return
		}

		// Try matching by NPC name
		for key, npc := range gs.NPCs {
			if strings.ToLower(npc.Name) == npcKey {
				if npc.Items == nil {
					npc.Items = make([]string, 0)
				}
				npc.Items = append(npc.Items, item)
				gs.NPCs[key] = npc // Write back
				break
			}
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
