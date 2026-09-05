package state

import (
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// handleNPCEvent processes an NPC state change event
func (a *Applier) handleNPCEvent(event conditionals.NPCEvent) {
	npcKey := strings.ToLower(strings.TrimSpace(event.NPCID))
	npc, npcExists := a.gs.NPCs[npcKey]
	if !npcExists {
		// Try matching by NPC name
		for key, n := range a.gs.NPCs {
			if strings.ToLower(n.Name) == npcKey {
				npcKey = key
				npc = n
				npcExists = true
				break
			}
		}
	}

	if !npcExists {
		if a.logger != nil {
			a.logger.Warn("NPC not found for event",
				"npc_id", event.NPCID)
		}
		return
	}

	modified := false

	// Handle location change
	if event.SetLocation != nil {
		locationKey := strings.ToLower(strings.TrimSpace(*event.SetLocation))
		_, locationExists := a.gs.WorldLocations[locationKey]

		if !locationExists {
			// Try matching by location name
			for key, loc := range a.gs.WorldLocations {
				if strings.ToLower(loc.Name) == locationKey {
					locationKey = key
					locationExists = true
					break
				}
			}
		}

		if locationExists {
			oldLocation := npc.Location
			npc.Location = locationKey
			modified = true

			if a.logger != nil {
				a.logger.Info("NPC location changed",
					"npc", npcKey,
					"from", oldLocation,
					"to", locationKey)
			}
		} else if a.logger != nil {
			a.logger.Warn("Location not found for NPC event",
				"npc_id", event.NPCID,
				"set_location", *event.SetLocation)
		}
	}

	// Handle following attribute
	if event.SetFollowing != nil {
		following := strings.TrimSpace(*event.SetFollowing)

		// Validate following target
		if following != "" && following != "pc" {
			// Should be a valid NPC ID
			_, exists := a.gs.NPCs[following]
			if !exists {
				// Try case-insensitive match
				found := false
				for key, n := range a.gs.NPCs {
					if strings.EqualFold(n.Name, following) {
						following = key
						found = true
						break
					}
				}
				if !found && a.logger != nil {
					a.logger.Warn("Following target not found",
						"npc", npcKey,
						"following", following)
				}
			}
		}

		npc.Following = following
		modified = true

		if a.logger != nil {
			a.logger.Info("NPC following changed",
				"npc", npcKey,
				"following", following)
		}
	}

	// Save changes
	if modified {
		a.gs.NPCs[npcKey] = npc
	}
}

// handleMonsterEvent processes a monster event (spawn or despawn)
func (a *Applier) handleMonsterEvent(event conditionals.MonsterEvent) {
	switch event.Action {
	case conditionals.MonsterEventSpawn:
		a.handleMonsterSpawn(event)
	case conditionals.MonsterEventDespawn:
		a.handleMonsterDespawn(event)
	default:
		a.logger.Warn("Unknown monster event action",
			"action", event.Action,
			"instance_id", event.InstanceID)
	}
}

// handleMonsterSpawn loads a monster template and spawns an instance
func (a *Applier) handleMonsterSpawn(event conditionals.MonsterEvent) {
	// Validate storage is available
	if a.storage == nil {
		a.logger.Error("Cannot spawn monster: storage not configured")
		return
	}

	// Normalize location key
	locationKey := strings.ToLower(strings.TrimSpace(event.Location))
	_, locationExists := a.gs.WorldLocations[locationKey]

	if !locationExists {
		// Try matching by location name
		for key, loc := range a.gs.WorldLocations {
			if strings.ToLower(loc.Name) == locationKey {
				locationKey = key
				locationExists = true
				break
			}
		}
	}

	if !locationExists {
		a.logger.Warn("Cannot spawn monster: location not found",
			"instance_id", event.InstanceID,
			"location", event.Location)
		return
	}

	// Load monster template from storage
	template, err := a.storage.GetMonster(a.ctx, event.Template)
	if err != nil {
		a.logger.Error("Failed to load monster template",
			"instance_id", event.InstanceID,
			"template", event.Template,
			"error", err)
		return
	}

	// Build monster definition from event (contains ID, location, and any overrides)
	monsterDef := &actor.Monster{
		ID:         event.InstanceID,
		TemplateID: event.Template,
		Location:   locationKey,
	}

	if event.Name != "" {
		monsterDef.Name = event.Name
	}
	if event.Description != "" {
		monsterDef.Description = event.Description
	}
	if event.AC != 0 {
		monsterDef.AC = event.AC
	}
	if event.HP != 0 {
		monsterDef.HP = event.HP
	}
	if event.MaxHP != 0 {
		monsterDef.MaxHP = event.MaxHP
	}
	if len(event.Attributes) > 0 {
		monsterDef.Attributes = event.Attributes
	}
	if len(event.CombatMods) > 0 {
		monsterDef.CombatMods = event.CombatMods
	}
	if len(event.Items) > 0 {
		monsterDef.Items = event.Items
	}
	if event.DropItemsOnDefeat != nil {
		monsterDef.DropItemsOnDefeat = *event.DropItemsOnDefeat
	}

	// Spawn the monster using GameState method
	monster := a.gs.SpawnMonster(template, monsterDef)

	if a.logger != nil {
		a.logger.Info("Monster spawned",
			"instance_id", event.InstanceID,
			"template", event.Template,
			"location", locationKey,
			"name", monster.Name)
	}
}

// handleMonsterDespawn removes a monster instance from the game
func (a *Applier) handleMonsterDespawn(event conditionals.MonsterEvent) {
	// Check if monster exists before despawning
	var exists bool
	for _, loc := range a.gs.WorldLocations {
		if _, found := loc.Monsters[event.InstanceID]; found {
			exists = true
			break
		}
	}

	if !exists {
		a.logger.Warn("Cannot despawn monster: instance not found", "instance_id", event.InstanceID)
		return
	}

	a.gs.DespawnMonster(event.InstanceID)
	a.logger.Info("Monster despawned", "instance_id", event.InstanceID)
}

// syncFollowingNPCs updates locations of NPCs that are following other actors
// This runs AFTER all other delta operations complete to ensure location changes are processed first.
// It iterates until convergence to correctly handle chained following (e.g. A follows B follows PC).
func (a *Applier) syncFollowingNPCs() {
	for {
		changed := false
		for npcKey, npc := range a.gs.NPCs {
			if npc.Following == "" {
				continue // Not following anyone
			}

			var targetLocation string

			if npc.Following == "pc" {
				// Following the player character
				targetLocation = a.gs.Location
			} else {
				// Following another NPC
				followedNPC, exists := a.gs.NPCs[npc.Following]
				if !exists {
					// Try case-insensitive match
					for _, n := range a.gs.NPCs {
						if strings.EqualFold(n.Name, npc.Following) {
							followedNPC = n
							exists = true
							break
						}
					}
				}

				if !exists {
					if a.logger != nil {
						a.logger.Warn("NPC following target not found",
							"npc", npcKey,
							"following", npc.Following)
					}
					continue
				}

				targetLocation = followedNPC.Location
			}

			// Update NPC location if it differs from target
			if npc.Location != targetLocation {
				oldLocation := npc.Location
				npc.Location = targetLocation
				a.gs.NPCs[npcKey] = npc
				changed = true

				if a.logger != nil {
					a.logger.Info("NPC location synced (following)",
						"npc", npcKey,
						"from", oldLocation,
						"to", targetLocation,
						"following", npc.Following)
				}
			}
		}
		if !changed {
			break
		}
	}
}
