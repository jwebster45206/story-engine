package state

import (
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// handleNPCEvent processes an NPC state change event
func (dw *DeltaWorker) handleNPCEvent(event conditionals.NPCEvent) {
	npcKey := strings.ToLower(strings.TrimSpace(event.NPCID))
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
			dw.logger.Warn("NPC not found for event",
				"npc_id", event.NPCID)
		}
		return
	}

	modified := false

	// Handle location change
	if event.SetLocation != nil {
		locationKey := strings.ToLower(strings.TrimSpace(*event.SetLocation))
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

		if locationExists {
			oldLocation := npc.Location
			npc.Location = locationKey
			modified = true

			if dw.logger != nil {
				dw.logger.Info("NPC location changed",
					"npc", npcKey,
					"from", oldLocation,
					"to", locationKey)
			}
		} else if dw.logger != nil {
			dw.logger.Warn("Location not found for NPC event",
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
			_, exists := dw.gs.NPCs[following]
			if !exists {
				// Try case-insensitive match
				found := false
				for key, n := range dw.gs.NPCs {
					if strings.EqualFold(n.Name, following) {
						following = key
						found = true
						break
					}
				}
				if !found && dw.logger != nil {
					dw.logger.Warn("Following target not found",
						"npc", npcKey,
						"following", following)
				}
			}
		}

		npc.Following = following
		modified = true

		if dw.logger != nil {
			dw.logger.Info("NPC following changed",
				"npc", npcKey,
				"following", following)
		}
	}

	// Save changes
	if modified {
		dw.gs.NPCs[npcKey] = npc
	}
}

// handleMonsterEvent processes a monster event (spawn or despawn)
func (dw *DeltaWorker) handleMonsterEvent(event conditionals.MonsterEvent) {
	switch event.Action {
	case conditionals.MonsterEventSpawn:
		dw.handleMonsterSpawn(event)
	case conditionals.MonsterEventDespawn:
		dw.handleMonsterDespawn(event)
	default:
		dw.logger.Warn("Unknown monster event action",
			"action", event.Action,
			"instance_id", event.InstanceID)
	}
}

// handleMonsterSpawn loads a monster template and spawns an instance
func (dw *DeltaWorker) handleMonsterSpawn(event conditionals.MonsterEvent) {
	// Validate storage is available
	if dw.storage == nil {
		dw.logger.Error("Cannot spawn monster: storage not configured")
		return
	}

	// Normalize location key
	locationKey := strings.ToLower(strings.TrimSpace(event.Location))
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
		dw.logger.Warn("Cannot spawn monster: location not found",
			"instance_id", event.InstanceID,
			"location", event.Location)
		return
	}

	// Load monster template from storage
	template, err := dw.storage.GetMonster(dw.ctx, event.Template)
	if err != nil {
		dw.logger.Error("Failed to load monster template",
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
	monster := dw.gs.SpawnMonster(template, monsterDef)

	if dw.logger != nil {
		dw.logger.Info("Monster spawned",
			"instance_id", event.InstanceID,
			"template", event.Template,
			"location", locationKey,
			"name", monster.Name)
	}
}

// handleMonsterDespawn removes a monster instance from the game
func (dw *DeltaWorker) handleMonsterDespawn(event conditionals.MonsterEvent) {
	// Check if monster exists before despawning
	var exists bool
	for _, loc := range dw.gs.WorldLocations {
		if _, found := loc.Monsters[event.InstanceID]; found {
			exists = true
			break
		}
	}

	if !exists {
		dw.logger.Warn("Cannot despawn monster: instance not found", "instance_id", event.InstanceID)
		return
	}

	dw.gs.DespawnMonster(event.InstanceID)
	dw.logger.Info("Monster despawned", "instance_id", event.InstanceID)
}

// syncFollowingNPCs updates locations of NPCs that are following other actors
// This runs AFTER all other delta operations complete to ensure location changes are processed first.
// It iterates until convergence to correctly handle chained following (e.g. A follows B follows PC).
func (dw *DeltaWorker) syncFollowingNPCs() {
	for {
		changed := false
		for npcKey, npc := range dw.gs.NPCs {
			if npc.Following == "" {
				continue // Not following anyone
			}

			var targetLocation string

			if npc.Following == "pc" {
				// Following the player character
				targetLocation = dw.gs.Location
			} else {
				// Following another NPC
				followedNPC, exists := dw.gs.NPCs[npc.Following]
				if !exists {
					// Try case-insensitive match
					for _, n := range dw.gs.NPCs {
						if strings.EqualFold(n.Name, npc.Following) {
							followedNPC = n
							exists = true
							break
						}
					}
				}

				if !exists {
					if dw.logger != nil {
						dw.logger.Warn("NPC following target not found",
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
				dw.gs.NPCs[npcKey] = npc
				changed = true

				if dw.logger != nil {
					dw.logger.Info("NPC location synced (following)",
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
