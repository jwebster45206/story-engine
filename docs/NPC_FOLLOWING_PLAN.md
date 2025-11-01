# NPC Following Feature - Implementation Plan

## Overview

Add support for NPCs to automatically follow other actors (PC or other NPCs). When an NPC is "following" another actor, the deterministic reducer layer will automatically sync the NPC's location to match the followed actor's location each turn.

---

## Phase 1: Add Following Attribute & Core Behavior

**Goal**: Add the `following` field to NPCs and implement automatic location syncing in the reducer.

### 1.1 Data Model Changes

**File**: `pkg/actor/npc.go`

Add `Following` field to the NPC struct:

```go
type NPC struct {
    Name               string                           `json:"name"`
    Type               string                           `json:"type"`
    Disposition        string                           `json:"disposition"`
    Description        string                           `json:"description,omitempty"`
    IsImportant        bool                             `json:"important,omitempty"`
    Location           string                           `json:"location,omitempty"`
    Following          string                           `json:"following,omitempty"`           // NEW: ID of actor being followed ("pc" or NPC ID)
    Items              []string                         `json:"items,omitempty"`
    ContingencyPrompts []conditionals.ContingencyPrompt `json:"contingency_prompts,omitempty"`
}
```

**Special values**:
- `"pc"` - NPC follows the player character
- `"<npc_id>"` - NPC follows another NPC (e.g., `"gibbs"`)
- `""` (empty/omitted) - NPC is not following anyone

### 1.2 Reducer Logic - Location Syncing

**File**: `pkg/state/deltaworker.go`

Add new method `syncFollowingNPCs()` that runs **after** `Apply()` completes.

**Important**: This must run after other location changes (including player movement and explicit NPC movements) to ensure we're syncing to the correct final locations:

```go
// syncFollowingNPCs updates locations of NPCs that are following other actors
// This runs AFTER all other delta operations complete to ensure location changes are processed first
func (dw *DeltaWorker) syncFollowingNPCs() {
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
                for key, n := range dw.gs.NPCs {
                    if strings.ToLower(n.Name) == strings.ToLower(npc.Following) {
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
            
            if dw.logger != nil {
                dw.logger.Info("NPC location synced (following)",
                    "npc", npcKey,
                    "from", oldLocation,
                    "to", targetLocation,
                    "following", npc.Following)
            }
        }
    }
}
```

**Update**: Modify `Apply()` method to call `syncFollowingNPCs()` **at the very end**, after all other operations:

```go
func (dw *DeltaWorker) Apply() error {
    // ... existing Apply logic (user location, scene changes, item events, NPC movements, game end) ...
    
    // Ensure that items are singletons
    dw.gs.NormalizeItems()
    
    // IMPORTANT: Sync locations for NPCs that are following other actors
    // This MUST be last to ensure we sync to final locations after all other changes
    dw.syncFollowingNPCs()
    
    return nil
}
```

**Order matters**: 
1. Player moves to new location
2. Explicit NPC movements happen (via NPCMovements or NPCEvents)
4. Following NPCs sync to updated locations (one pass only, no recursion)
3. Items normalized


### 1.3 Documentation

**File**: `data/scenarios/README.md`

Add section under "NPCs (Non-Player Characters)" → "NPC Fields":

```markdown
### NPC Fields

- **type**: Role/profession of the NPC
- **disposition**: Personality and attitude toward the player
- **description**: Physical appearance and notable characteristics
- **location**: Current location of the NPC (use location ID)
- **following**: ID of actor this NPC follows automatically ("pc" for player, or another NPC's ID)
- **items**: Objects this NPC possesses
- **contingency_prompts**: NPC-specific prompts shown when at player location

### Following Behavior

NPCs can be set to automatically follow the player character or another NPC. When an NPC has a `following` value, the game engine automatically syncs their location each turn.

**Following the player:**
```json
"npcs": {
  "loyal_companion": {
    "name": "Loyal Companion",
    "type": "follower",
    "disposition": "devoted",
    "following": "pc",
    "location": "starting_area"
  }
}
```

**Following another NPC:**
```json
"npcs": {
  "bodyguard": {
    "name": "Captain's Bodyguard",
    "following": "pirate_captain",
    "location": "ship_deck"
  },
  "pirate_captain": {
    "name": "Pirate Captain",
    "location": "ship_deck"
  }
}
```

**How it works:**
- The `following` field is set in the scenario JSON
- Each turn, after all location changes are processed, NPCs following other actors have their locations automatically updated
- If the player moves from `tavern` to `market`, any NPC with `"following": "pc"` will also move to `market`
- Following NPCs move silently - the narrator doesn't need to explicitly describe them following

**Use cases:**
- Loyal companions that stay with the player
- Bodyguards or servants that follow a key NPC
- Pets or familiars
- Enforcers tracking a suspect
- Any NPC that should mirror another actor's movements
```

Add to "Contingency Rules" section:

```markdown
**NPC Following:**
- "To make Gibbs follow the player, set his following attribute to 'pc'"
- "When the quest completes, clear Gibbs' following attribute"
```

### 1.4 Validation

**File**: `cmd/validate/main.go`

Add validation in `validateScenario()` and `validateScene()` to check `following` values:

```go
// Validate NPC IDs and their following references
for npcID, npc := range s.NPCs {
    v.validateIDFormat("NPC ID", npcID)
    
    // Validate following field if present
    if npc.Following != "" {
        if npc.Following != "pc" {
            // Should reference another NPC
            v.validateIDFormat("NPC following", npc.Following)
        }
    }
    
    // Validate contingency prompts
    for _, cp := range npc.ContingencyPrompts {
        v.validateContingencyPrompt(&cp)
    }
}
```

### 1.5 Unit Tests

**File**: `pkg/state/deltaworker_following_test.go` (new file)

Create comprehensive unit tests:

```go
func TestDeltaWorker_SyncFollowingNPCs_FollowPC(t *testing.T)
func TestDeltaWorker_SyncFollowingNPCs_FollowAnotherNPC(t *testing.T)
func TestDeltaWorker_SyncFollowingNPCs_ChainedFollowing(t *testing.T) // NPC1 follows PC, NPC2 follows NPC1
func TestDeltaWorker_SyncFollowingNPCs_NoFollowing(t *testing.T)
func TestDeltaWorker_SyncFollowingNPCs_TargetNotFound(t *testing.T)
func TestDeltaWorker_SyncFollowingNPCs_AlreadyAtTargetLocation(t *testing.T)
func TestDeltaWorker_SyncFollowingNPCs_CaseInsensitiveMatch(t *testing.T)
```

### 1.6 Integration Tests

**File**: `integration/cases/pirate_npc_following.json` (new file)

Create integration test scenario:

```json
{
  "name": "Pirate NPC Following Test",
  "scenario": "x.pirate_following_test.json",
  "seed_game_state": {
    "scenario": "x.pirate_following_test.json",
    "chat_history": [
      {
        "role": "assistant",
        "content": "You stand on the docks with Gibbs at your side, loyal as always."
      }
    ]
  },
  "steps": [
    {
      "name": "verify gibbs follows to sleepy mermaid",
      "user_prompt": "I go to the Sleepy Mermaid tavern.",
      "expect": {
        "user_location": "sleepy_mermaid",
        "npcs": {
          "gibbs": {
            "location": "sleepy_mermaid"
          }
        }
      }
    },
    {
      "name": "verify gibbs follows to black pearl",
      "user_prompt": "I return to the Black Pearl.",
      "expect": {
        "user_location": "black_pearl",
        "npcs": {
          "gibbs": {
            "location": "black_pearl"
          }
        }
      }
    },
    {
      "name": "ask about gibbs presence",
      "user_prompt": "Is Gibbs here with me?",
      "expect": {
        "user_location": "black_pearl"
      },
      "response_contains": ["Gibbs"]
    }
  ]
}
```

**Test scenario file**: `data/scenarios/x.pirate_following_test.json`

```json
{
  "name": "Pirate Following Test",
  "story": "Test scenario for NPC following behavior.",
  "rating": "PG",
  "narrator_id": "comedic",
  "default_pc": "pirate_captain",
  "opening_scene": "main",
  "opening_location": "tortuga",
  "scenes": {
    "main": {
      "story": "Testing NPC following mechanics.",
      "locations": {
        "tortuga": {
          "name": "Tortuga",
          "exits": {
            "south": "sleepy_mermaid",
            "east": "black_pearl"
          }
        },
        "sleepy_mermaid": {
          "name": "Sleepy Mermaid",
          "exits": {"north": "tortuga"}
        },
        "black_pearl": {
          "name": "Black Pearl",
          "exits": {"west": "tortuga"}
        }
      },
      "npcs": {
        "gibbs": {
          "name": "Gibbs",
          "type": "first_mate",
          "disposition": "loyal",
          "description": "Your loyal first mate",
          "location": "tortuga",
          "following": "pc"
        }
      }
    }
  }
}
```

### 1.7 Phase 1 Checklist

- [ ] Add `Following` field to `pkg/actor/npc.go`
- [ ] Implement `syncFollowingNPCs()` in `pkg/state/deltaworker.go`
- [ ] Update `Apply()` to call `syncFollowingNPCs()`
- [ ] Add documentation to `data/scenarios/README.md`
- [ ] Add validation in `cmd/validate/main.go`
- [ ] Create unit tests in `pkg/state/deltaworker_following_test.go`
- [ ] Create integration test `integration/cases/pirate_npc_following.json`
- [ ] Create test scenario `data/scenarios/x.pirate_following_test.json`
- [ ] Run all tests (unit + integration)
- [ ] Verify no regressions

---

## Phase 2: Refactor NPCMovements to NPCEvents

**Goal**: Replace the `NPCMovements` array in `GameStateDelta` with a more flexible `NPCEvents` structure that can handle location assignment AND following attribute changes.

**Why Phase 2 is separate from Phase 1**: 
- Phase 1 establishes the core following behavior (NPCs automatically follow in the reducer)
- Phase 2 adds the ability to **change** following behavior via conditionals and LLM actions
- They can be implemented and tested independently

### 2.1 Data Model Changes

**File**: `pkg/conditionals/delta.go`

**Replace** `NPCMovement` with `NPCEvent` (breaking change):

```go
// GameStateDelta represents changes to apply to game state
type GameStateDelta struct {
    UserLocation string        `json:"user_location,omitempty"`
    SceneChange  *SceneChange  `json:"scene_change,omitempty"`
    ItemEvents   []interface{} `json:"item_events,omitempty"`
    
    // NPCEvents replaces NPCMovements (breaking change in v0)
    NPCEvents []NPCEvent `json:"npc_events,omitempty"`
    
    SetVars   map[string]string `json:"set_vars,omitempty"`
    GameEnded *bool             `json:"game_ended,omitempty"`
    Prompt    *string           `json:"prompt,omitempty"`
}

// NPCEvent represents a change to an NPC's state
type NPCEvent struct {
    NPCID        string  `json:"npc_id"`
    SetLocation  *string `json:"set_location,omitempty"`  // Set NPC to specific location
    SetFollowing *string `json:"set_following,omitempty"` // Set following target ("pc", npc_id, or "" to clear); nil = no change
}
```

**Design notes**:
- Use pointers for optional fields (`*string`) to distinguish between "not set" and "set to empty"
- `SetFollowing` set to `""` clears the following attribute
- `SetFollowing` set to `nil` means don't change the following attribute
- Multiple fields can be set in a single event (e.g., move NPC AND make them follow)

**Remove entirely**:
- `NPCMovements []NPCMovement` field
- `NPCMovement` struct
- Any references to `npc_movements` in the codebase

### 2.2 Reducer Logic Updates

**File**: `pkg/state/deltaworker.go`

Replace `handleNPCMovement()` with `handleNPCEvent()`:

```go
// handleNPCEvent processes an NPC state change event
func (dw *DeltaWorker) handleNPCEvent(event conditionals.NPCEvent) {
    // Normalize the NPC identifier
    npcKey := strings.ToLower(strings.TrimSpace(event.NPCID))
    
    // Find the NPC
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
                    if strings.ToLower(n.Name) == strings.ToLower(following) {
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
```

**Update**: Modify `Apply()` to process NPCEvents (replacing the old NPCMovements loop):

```go
func (dw *DeltaWorker) Apply() error {
    // ... existing code for user location, scene changes, item events ...
    
    // Handle NPC events (REPLACES the old "Handle NPC movements" section)
    for _, npcEvent := range dw.delta.NPCEvents {
        dw.handleNPCEvent(npcEvent)
    }
    
    // Handle Game End
    if dw.delta.GameEnded != nil && *dw.delta.GameEnded {
        dw.gs.IsEnded = true
    }
    
    // Ensure that items are singletons
    dw.gs.NormalizeItems()
    
    // Sync locations for NPCs that are following other actors 
    // (could override NPCEvent movements)
    dw.syncFollowingNPCs()
    
    return nil
}
```

**Note**: The NPC events are processed **before** `syncFollowingNPCs()`, so if an event changes an NPC's following status, the sync will use the updated value.

**Update**: Modify `mergeDelta()` to handle NPCEvents:

```go
func (dw *DeltaWorker) mergeDelta(conditionalDelta *conditionals.GameStateDelta, conditionalID string) {
    // ... existing code ...
    
    // Merge NPC events
    if len(conditionalDelta.NPCEvents) > 0 {
        dw.delta.NPCEvents = append(dw.delta.NPCEvents, conditionalDelta.NPCEvents...)
    }
    
    // ... rest of mergeDelta logic ...
}
```

### 2.3 Schema Updates for LLM

**File**: `internal/services/anthropic.go`

Update `getDeltaUpdateTool()` to use `npc_events` instead of `npc_movements`:

```go
"npc_events": map[string]any{
    "type": "array",
    "items": map[string]any{
        "type": "object",
        "additionalProperties": false,
        "properties": map[string]any{
            "npc_id": map[string]any{
                "type": "string",
            },
            "set_location": map[string]any{
                "type": "string",
            },
        },
        "required": []string{"npc_id"},
    },
},
```

**Note**: The `set_following` and `clear_following` fields are **NOT** exposed to the LLM. The reducer only handles explicit location changes. Following behavior is controlled through:
1. Initial scenario JSON configuration
2. Conditional rules (which can use `set_following` and `clear_following`)

**File**: `pkg/prompts/prompts.go`

Update `ReducerPrompt` to describe `npc_events`:

```go
OUTPUT SCHEMA (strict)
- user_location: string (always required)
- scene_change: object { to, reason } or null when no change
- item_events: array of { item, action, from?, to?, consumed?, evidence? }
- npc_events: array of { npc_id, set_location }
- set_vars: object (always required, may be empty)
- game_ended: boolean (always required)

NPC EVENTS
- Use npc_events to track explicit NPC location changes
- set_location: Move NPC to a specific location
  • "Gibbs follows you into the tavern." → npc_events:[{npc_id:"gibbs", set_location:"sleepy_mermaid"}]
  • "The guard rushes to the throne room." → npc_events:[{npc_id:"guard", set_location:"throne_room"}]
- Note: NPCs with a "following" attribute will automatically sync their location after all events are processed
```

### 2.4 Documentation Updates ✅

**File**: `data/scenarios/README.md`

Update conditional examples to use `npc_events`:

```markdown
**NPC Events (Location Changes):**
```json
"then": {
  "npc_events": [
    {
      "npc_id": "guard",
      "set_location": "throne_room"
    }
  ]
}
```

**NPC Events (Following Behavior - Conditionals Only):**
```json
"then": {
  "npc_events": [
    {
      "npc_id": "loyal_companion",
      "set_following": "pc"
    }
  ]
}
```

**Combined location and following:**
```json
"then": {
  "npc_events": [
    {
      "npc_id": "gibbs",
      "set_location": "black_pearl",
      "set_following": "pc"
    }
  ]
}
```

**Stop following (set to empty string):**
```json
"then": {
  "npc_events": [
    {
      "npc_id": "gibbs",
      "set_following": ""
    }
  ]
}
```

**Important**: The LLM reducer can only emit `set_location` in `npc_events`. The `set_following` field is only available in conditional rules defined in scenario JSON.
```

### 2.5 Validation Updates ✅

**File**: `cmd/validate/main.go`

Update to validate `NPCEvents` instead of `NPCMovements`:

```go
if len(conditional.Then.NPCEvents) > 0 {
    for _, npcEvent := range conditional.Then.NPCEvents {
        // Validate NPC ID format
        if npcEvent.NPCID != "" {
            v.validateIDFormat("npc_event npc_id", npcEvent.NPCID)
        }
        
        // Validate location if set
        if npcEvent.SetLocation != nil && *npcEvent.SetLocation != "" {
            v.validateIDFormat("npc_event set_location", *npcEvent.SetLocation)
        }
        
        // Validate following if set
        if npcEvent.SetFollowing != nil {
            following := *npcEvent.SetFollowing
            if following != "" && following != "pc" {
                v.validateIDFormat("npc_event set_following", following)
            }
        }
    }
    actionCount++
}
```

### 2.6 Unit Tests ✅

**File**: `pkg/state/deltaworker_npcevents_test.go` (new file)

Create tests for NPCEvent handling:

```go
func TestDeltaWorker_HandleNPCEvent_SetLocation(t *testing.T)
func TestDeltaWorker_HandleNPCEvent_SetFollowing(t *testing.T)
func TestDeltaWorker_HandleNPCEvent_ClearFollowing(t *testing.T)
func TestDeltaWorker_HandleNPCEvent_CombinedLocationAndFollowing(t *testing.T)
func TestDeltaWorker_HandleNPCEvent_InvalidNPC(t *testing.T)
func TestDeltaWorker_HandleNPCEvent_InvalidLocation(t *testing.T)
func TestDeltaWorker_HandleNPCEvent_InvalidFollowingTarget(t *testing.T)
func TestDeltaWorker_HandleNPCEvent_CaseInsensitiveMatching(t *testing.T)
```

### 2.7 Integration Tests ✅

Update the scenarios used in all integrations tests for compatibility with NPC Events. 

- pirate.json
- space-disaster.json
- dracula.json

Run the validator on the scenarios.

Update the pirate scenario to add a conditional-based follow. 

- Set the scene to scene 2 "british docks", and set the player in Tortuga.
- Make sure `davey` is seeded, and chat history indicates that Calypso congratulated player on hiring shipwright, and told him to seek out davey in Tortuga. 
- Please make other aspects of the seeding realistic for a "british docks" scene.
- Set up a contingency_rule to activate a "davey_recruited" var when Davey agrees to help or follow the player.
- In prompt, player should look for davey. (davey following should not change)
- In prompt, player should ask for Davey's help infiltrating the British. docks, hinting there's a bit of coin in it for him. (davey following should activate)
- In prompt, player should move to black pearl. (assert that davey's location changes too)

Put this new integ test in a new file:

**File**: `integration/cases/pirate_npc_events.json` (new file)

### 2.8 Breaking Changes (v0)

Since we're in v0, we can make clean breaking changes:

**Files to modify**:
1. `pkg/conditionals/delta.go` - Delete `NPCMovement` struct, delete `NPCMovements` field, add `NPCEvents` field
2. `pkg/state/deltaworker.go` - Delete `handleNPCMovement()`, add `handleNPCEvent()`
3. `internal/services/anthropic.go` - Replace `npc_movements` schema with `npc_events` schema
4. `pkg/prompts/prompts.go` - Replace `npc_movements` instructions with `npc_events` instructions
5. `cmd/validate/main.go` - Replace `NPCMovements` validation with `NPCEvents` validation
6. All test files - Update any tests using `npc_movements` to use `npc_events`
7. `data/scenarios/README.md` - Update documentation examples

**Search and replace strategy**:
- `NPCMovements` → `NPCEvents` (struct field name)
- `npc_movements` → `npc_events` (JSON field name)
- `handleNPCMovement` → `handleNPCEvent` (method name)
- `NPCMovement` → `NPCEvent` (struct type name)

**Files that reference NPCMovement** (grep for verification):
```bash
grep -r "NPCMovement" --include="*.go" .
grep -r "npc_movement" --include="*.go" --include="*.md" .
```

**No migration needed** - this is a clean break for v0. Old scenarios using `npc_movements` in conditionals will break and need to be updated.

### 2.9 Phase 2 Checklist

- [x] **Delete** `NPCMovement` struct and `NPCMovements` field from `pkg/conditionals/delta.go`
- [x] **Create** `NPCEvent` struct in `pkg/conditionals/delta.go`
- [x] **Replace** `handleNPCMovement()` with `handleNPCEvent()` in `pkg/state/deltaworker.go`
- [x] Update `Apply()` to use `NPCEvents`
- [x] Update `mergeDelta()` to use `NPCEvents`
- [x] Update LLM schema in `internal/services/anthropic.go` (replace `npc_movements` with `npc_events`)
- [x] Update LLM schema in `internal/services/venice.go` (replace `npc_movements` with `npc_events`)
- [x] Update reducer prompts in `pkg/prompts/prompts.go`
- [x] Update documentation in `data/scenarios/README.md`
- [x] Update validation in `cmd/validate/main.go` (delete `NPCMovements` validation)
- [ ] Update/delete existing unit tests referencing `NPCMovement`
- [x] Create unit tests in `pkg/state/deltaworker_npcevents_test.go`
- [x] Create integration test `integration/cases/pirate_npc_events.json`
- [x] Update pirate.json scenario with `davey_follows_player` conditional
- [ ] Run all tests (unit + integration)
- [x] Search codebase for any remaining `NPCMovement` references and remove

---

## Testing Strategy

### Phase 1 Testing Focus
- NPCs correctly follow the PC through location changes
- NPCs correctly follow other NPCs through location changes  
- Chained following (NPC A follows B, B follows PC)
- Following behavior doesn't interfere with explicit NPC movements
- Following attribute persists through save/load
- Validation catches invalid following targets

### Phase 2 Testing Focus
- `npc_events` correctly sets location (via LLM reducer)
- `npc_events` correctly sets following (via conditionals only)
- `npc_events` correctly clears following (via conditionals only)
- Combined location + following in single conditional event
- Conditionals trigger NPC events correctly
- LLM correctly emits `npc_events` with `set_location` only in reducer output
- LLM does NOT emit `set_following` or `clear_following` (not exposed in schema)
- Validation catches malformed `npc_events`
- All old `npc_movements` references removed

---

## Success Criteria

### Phase 1
✅ NPCs with `"following": "pc"` automatically move with the player  
✅ NPCs can follow other NPCs  
✅ Location syncing happens deterministically each turn  
✅ Documentation clearly explains the feature  
✅ Integration tests pass demonstrating following behavior  
✅ No regressions in existing NPC movement behavior  

### Phase 2
✅ `npc_events` completely replaces `npc_movements` (breaking change)  
✅ NPCs can be assigned locations via conditionals and LLM reducer  
✅ NPCs can start/stop following via conditionals only (not exposed to LLM)  
✅ LLM reducer correctly emits `npc_events` with `set_location` only  
✅ All old `npc_movements` code removed  
✅ All tests pass (unit + integration)  
✅ Documentation updated with new examples  

---

## Future Enhancements (Out of Scope)

- **Following chains validation**: Detect circular following (A follows B, B follows A)
- **Following distance**: NPCs that follow but stay 1-2 locations behind
- **Conditional following**: NPCs that only follow in certain situations
- **Following groups**: Multiple NPCs following as a group
- **Following events**: Trigger story events when following starts/stops
- **Following prompts**: Contingency prompts that only show when following
