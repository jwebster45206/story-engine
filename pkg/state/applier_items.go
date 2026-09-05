package state

import (
	"slices"
	"strings"
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

// handleAcquireItem adds an item to player inventory
func (a *Applier) handleAcquireItem(itemEvent itemEvent) {
	if !slices.Contains(a.gs.Inventory, itemEvent.Item) {
		if a.gs.Inventory == nil {
			a.gs.Inventory = make([]string, 0)
		}
		a.gs.Inventory = append(a.gs.Inventory, itemEvent.Item)
	}
	// Remove from source if specified and not consumed
	if itemEvent.From != nil && (itemEvent.Consumed == nil || !*itemEvent.Consumed) {
		a.removeItemFromSource(itemEvent.Item, itemEvent.From)
	}
}

// handleDropItem removes an item from player inventory
func (a *Applier) handleDropItem(itemEvent itemEvent) {
	for i, invItem := range a.gs.Inventory {
		if invItem == itemEvent.Item {
			a.gs.Inventory = append(a.gs.Inventory[:i], a.gs.Inventory[i+1:]...)
			break
		}
	}
	// Add to destination if specified
	if itemEvent.To != nil {
		a.addItemToDestination(itemEvent.Item, itemEvent.To)
	}
}

// handleGiveItem transfers an item between entities
func (a *Applier) handleGiveItem(itemEvent itemEvent) {
	// Remove from source
	if itemEvent.From != nil {
		a.removeItemFromSource(itemEvent.Item, itemEvent.From)
	} else {
		// Default to removing from player inventory if no source specified
		for i, invItem := range a.gs.Inventory {
			if invItem == itemEvent.Item {
				a.gs.Inventory = append(a.gs.Inventory[:i], a.gs.Inventory[i+1:]...)
				break
			}
		}
	}
	// Add to destination
	if itemEvent.To != nil {
		a.addItemToDestination(itemEvent.Item, itemEvent.To)
	}
}

// handleMoveItem moves an item from one location/entity to another
func (a *Applier) handleMoveItem(itemEvent itemEvent) {
	// Remove from source
	if itemEvent.From != nil {
		a.removeItemFromSource(itemEvent.Item, itemEvent.From)
	}
	// Add to destination
	if itemEvent.To != nil {
		a.addItemToDestination(itemEvent.Item, itemEvent.To)
	}
}

// handleUseItem uses an item and potentially consumes it
func (a *Applier) handleUseItem(itemEvent itemEvent) {
	// If item is consumed, remove it from source
	if itemEvent.Consumed != nil && *itemEvent.Consumed {
		if itemEvent.From != nil {
			a.removeItemFromSource(itemEvent.Item, itemEvent.From)
		} else {
			// Default to removing from player inventory if no source specified
			for i, invItem := range a.gs.Inventory {
				if invItem == itemEvent.Item {
					a.gs.Inventory = append(a.gs.Inventory[:i], a.gs.Inventory[i+1:]...)
					break
				}
			}
		}
	}
}

// removeItemFromSource removes an item from the specified source
func (a *Applier) removeItemFromSource(item string, from *struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}) {
	gs := a.gs
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
func (a *Applier) addItemToDestination(item string, to *struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}) {
	gs := a.gs
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
