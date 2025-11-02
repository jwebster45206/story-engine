# Story Engine — Monster System (v1)

**Goal:** Introduce first-class **monsters** (mobs/creatures) with lifecycle control, d20-compatible stats, and deterministic spawn/despawn behavior — *combat-lite*: the engine may attach pre-rolled outcomes to user messages, but core focus here is monster data + lifecycle.

**Key Design Decisions (Revised):**
- **External JSON templates**: Monster templates defined in `data/monsters/` directory with storage layer support
- **Template vs Instance**: Templates are reusable definitions; instances are spawned copies with unique IDs
- **Explicit instance IDs**: Scenario writers assign instance IDs for precise conditional control
- **No HP rolling**: Monsters have fixed HP values (no dice notation)
- **Flexible attributes**: Monsters use a simple `attributes` map (not full 5e stats)
- **No XP system**: XP tracking removed entirely
- **No death prompts**: Complexity omitted for v1
- **DeltaWorker integration**: Spawn/despawn handled in conditional evaluation flow
- **API endpoints**: Monster templates accessible via REST API (like PCs and narrators)

---

## Table of Contents
1. [Design Principles](#design-principles)  
2. [Actor Layer — Monster Model](#actor-layer--monster-model)  
   - [Struct](#struct)  
   - [Creation & Destruction](#creation--destruction)  
   - [D20 Integration](#d20-integration)  
   - [Receiver Methods](#receiver-methods)  
   - [Edge Cases](#edge-cases)  
3. [State Layer — GameState Lifecycle](#state-layer--gamestate-lifecycle)  
   - [GameState Additions](#gamestate-additions)  
   - [Spawn / Despawn](#spawn--despawn)  
   - [Automatic Defeat Detection](#automatic-defeat-detection)  
4. [Scenario JSON — Declaring Monsters](#scenario-json--declaring-monsters)  
   - [Pre-placed (inline) Monsters](#pre-placed-inline-monsters)  
   - [Conditional Spawn/Despawn](#conditional-spawndespawn)  
5. [Prompt Builder & LLM Rules](#prompt-builder--llm-rules)  
6. [API & Schema Updates (OpenAPI)](#api--schema-updates-openapi)  
7. [Implementation Plan (Checklist)](#implementation-plan-checklist)  
8. [Unit Tests](#unit-tests)  
9. [Integration Tests](#integration-tests)  
10. [Examples](#examples)  
11. [Appendix — Pseudocode](#appendix--pseudocode)

---

## Design Principles

- **Monsters are data-first.** Narrative describes them, but creation/destruction is **system-controlled**, not LLM-created.
- **Lifecycle is deterministic.** Spawn/despawn happen via the **conditional reducer** (same layer as scene changes and story events) or at scenario init.
- **Defeat is mechanical.** The reducer handles monster defeat automatically when HP ≤ 0 (no scenario vars required).
- **Reuse d20.** Use the existing dice/actor library for HP rolls and attack/skill roll plumbing.
- **Backwards compatible.** Scenarios can ignore monsters entirely; nothing breaks.

---

## Actor Layer — Monster Model

Location: `pkg/actor/monster.go`

### Struct

```go
type Monster struct {
    ID          string         `json:"id"`
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Location    string         `json:"location"`

    AC          int            `json:"ac"`
    HP          int            `json:"hp"`
    MaxHP       int            `json:"max_hp"`

    Attributes  map[string]int `json:"attributes,omitempty"`      // Flexible key-value attributes (e.g., "strength": 16)
    CombatMods  map[string]int `json:"combat_modifiers,omitempty"`
    Items       []string       `json:"items,omitempty"`

    // defeat behavior
    DropItemsOnDefeat bool `json:"drop_items_on_defeat,omitempty"`
}
```

### Creation & Destruction

```go
func NewMonster(id string, base *Monster, location string) *Monster {
    m := *base // shallow copy of base template
    m.ID = id
    m.Location = location

    // HP is set directly from the template (no rolling)
    if m.MaxHP > 0 && m.HP == 0 {
        m.HP = m.MaxHP
    }
    return &m
}
```

**Note**: No `Destroy()` method needed - just delete from the map when defeated.

### D20 Integration

Monsters use the existing d20 library indirectly (via their stats), but do not need dedicated roll helpers at this time. Combat mechanics will be added later when needed.



### Receiver Methods

```go
func (m *Monster) TakeDamage(n int) {
    if n <= 0 { return }
    m.HP -= n
    if m.HP < 0 { m.HP = 0 }
}

func (m *Monster) Heal(n int) {
    if n <= 0 { return }
    m.HP += n
    if m.HP > m.MaxHP { m.HP = m.MaxHP }
}

func (m *Monster) IsDefeated() bool { return m.HP <= 0 }
func (m *Monster) MoveTo(loc string) { m.Location = loc }
```

### Edge Cases
- `AC`/`HP` must be non-negative; clamp at 0+.
- Monstrous "speech" is narrator-level choice; model stays silent by default.

---

## State Layer — GameState Lifecycle

Location: `pkg/state/gamestate.go`

### GameState Additions

```go
type GameState struct {
    // existing fields...
    Monsters map[string]*actor.Monster `json:"monsters,omitempty"`
}
```

### Spawn / Despawn

**Note**: The `instanceID` parameter is the unique ID for this monster instance (e.g., `"rat_1"`). The `template` parameter is the loaded Monster template from `data/monsters/{template_id}.json`.

```go
func (gs *GameState) SpawnMonster(instanceID string, template *actor.Monster, location string) *actor.Monster {
    if gs.Monsters == nil { gs.Monsters = map[string]*actor.Monster{} }

    m := actor.NewMonster(instanceID, template, location)
    gs.Monsters[instanceID] = m
    return m
}

func (gs *GameState) DespawnMonster(instanceID string) {
    m, ok := gs.Monsters[instanceID]
    if !ok { return }

    // Drop items to location
    if m.DropItemsOnDefeat && len(m.Items) > 0 {
        if loc, ok := gs.WorldLocations[m.Location]; ok {
            loc.Items = append(loc.Items, m.Items...)
            gs.WorldLocations[m.Location] = loc
        }
    }

    // Remove from map
    delete(gs.Monsters, instanceID)
}
```

### Automatic Defeat Detection

Call after each turn/reducer pass (especially after any damage change):

```go
func (gs *GameState) EvaluateDefeats() {
    for id, m := range gs.Monsters {
        if m.IsDefeated() {
            gs.DespawnMonster(id)
        }
    }
}
```

---

## Storage Layer — Monster Templates

Location: `internal/storage/storage.go` (interface), `internal/storage/redis.go` (implementation)

Monster templates are stored as external JSON files in `data/monsters/` and loaded on demand by the storage layer.

### Storage Interface

```go
type Storage interface {
    // existing methods...
    
    // GetMonster loads a monster template by ID from data/monsters/{id}.json
    GetMonster(ctx context.Context, templateID string) (*actor.Monster, error)
    
    // ListMonsters returns a map of monster names to their template IDs
    ListMonsters(ctx context.Context) (map[string]string, error)
}
```

### Implementation

```go
func (r *RedisStorage) GetMonster(ctx context.Context, templateID string) (*actor.Monster, error) {
    path := filepath.Join(r.dataDir, "monsters", templateID + ".json")
    
    file, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, fmt.Errorf("monster template not found: %s", templateID)
        }
        return nil, fmt.Errorf("failed to read monster template: %w", err)
    }
    
    var m actor.Monster
    if err := json.Unmarshal(file, &m); err != nil {
        return nil, fmt.Errorf("failed to unmarshal monster template: %w", err)
    }
    
    return &m, nil
}

func (r *RedisStorage) ListMonsters(ctx context.Context) (map[string]string, error) {
    monstersDir := filepath.Join(r.dataDir, "monsters")
    monsters := make(map[string]string)
    
    err := filepath.WalkDir(monstersDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
            return nil
        }
        
        file, err := os.ReadFile(path)
        if err != nil {
            r.logger.Warn("Failed to read monster file", "path", path, "error", err)
            return nil
        }
        
        var m actor.Monster
        if err := json.Unmarshal(file, &m); err != nil {
            r.logger.Warn("Failed to unmarshal monster file", "path", path, "error", err)
            return nil
        }
        
        templateID := strings.TrimSuffix(filepath.Base(path), ".json")
        monsters[m.Name] = templateID
        return nil
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to list monsters: %w", err)
    }
    
    return monsters, nil
}
```

---

## Scenario JSON — Declaring Monsters

### Monster Templates (External JSON)

Monsters are defined in **external JSON files** in `data/monsters/` directory, similar to how NPCs and locations work. Each monster file contains a reusable template:

```json
{
  "id": "giant_rat",
  "name": "Giant Rat",
  "description": "A filthy, red-eyed rodent the size of a dog.",
  "ac": 12,
  "hp": 9,
  "max_hp": 9,
  "attributes": {
    "strength": 7,
    "dexterity": 15,
    "constitution": 11
  },
  "combat_modifiers": {
    "bite": 4
  },
  "drop_items_on_defeat": true,
  "items": ["rat_pelt"]
}
```

### Pre-placed Monsters in Locations

Locations define a map of **instance IDs** to **template IDs**. At **scenario load time**, these monsters are automatically spawned:

```json
"locations": {
  "cellar": {
    "name": "Dark Cellar",
    "description": "A dank underground storage room.",
    "exits": {
      "up": "tavern"
    },
    "items": ["torch"],
    "monsters": {
      "rat_1": "giant_rat",
      "rat_2": "giant_rat"
    }
  }
}
```

**Key/Value Format**:
- **Key**: Instance ID (unique identifier for this specific monster instance, e.g., `"rat_1"`)
- **Value**: Template ID (references `data/monsters/{template_id}.json`, e.g., `"giant_rat"`)

This allows scenario writers to:
- Create multiple instances from the same template
- Give each instance a meaningful ID for conditional targeting
- Avoid auto-generated IDs that are hard to reference

When the scenario loads, the engine:
1. Iterates through each location's `monsters` map
2. For each entry, loads the template from `data/monsters/{template_id}.json`
3. Calls `GameState.SpawnMonster(instanceID, template, location)`
4. Places the instance at the specified location

**Where this happens**: In `internal/handlers/gamestate.go`, the `CreateGameState` handler initializes a new game. After setting up NPCs and locations, it should iterate through all locations and spawn any pre-placed monsters. This happens once at scenario initialization, similar to how NPCs are copied from the scenario to game state.

### Conditional Spawn/Despawn

Use the existing deterministic conditional system. Extend `then` with `spawn` / `despawn`:

```json
"conditionals": {
  "spawn_skeletons": {
    "when": { "vars": { "entered_graveyard": "true" } },
    "then": {
      "spawn": [
        {"instance_id": "skeleton_guard_1", "template": "skeleton", "location": "graveyard"},
        {"instance_id": "skeleton_guard_2", "template": "skeleton", "location": "graveyard"}
      ],
      "prompt": "STORY EVENT: Skeletons claw their way out of the dirt!"
    }
  },
  "cleanup_graveyard": {
    "when": { "vars": { "night_ended": "true" } },
    "then": {
      "despawn": ["skeleton_guard_1", "skeleton_guard_2"]
    }
  }
}
```

**Spawn Action Format**:
- `instance_id` (required): Unique ID for this monster instance in GameState
- `template` (required): Template ID to load from `data/monsters/{template}.json`
- `location` (required): Where to spawn the monster

**Despawn Action Format**:
- Array of instance IDs (strings) to remove from GameState

This design allows scenario writers to:
- Spawn multiple instances from the same template with explicit IDs
- Reference specific instances in later conditionals (e.g., for despawning)
- Avoid ambiguity about which monsters are affected by conditional actions

**Implementation Details:**
- Conditionals are evaluated by `DeltaWorker` in `pkg/state/deltaworker.go`
- `DeltaWorker.MergeConditionals()` evaluates scenario conditionals and merges triggered actions into the delta
- Spawn/despawn actions will be handled in the `ApplyConditionalActions()` or similar method
- The conditional system already handles `prompt`, `set_vars`, `scene_change`, `item_transfer`, etc.
- Monster spawn/despawn will follow the same pattern as existing conditional actions

---

## Prompt Builder & LLM Rules

- **Monsters at player's location** are included in the prompt context (similar to NPCs).  
- **LLM must not create monsters.** Only narrate monsters that exist in game state.
- Suggested system guidance:
  - "Monsters are predefined by the engine. Do **not** invent or add monsters that are not present in game state. Describe and react to monsters that are active."
  - "Monsters generally do not speak (unless obviously sentient/magical); prefer physical action descriptions."

Implementation:
- Update `pkg/prompts/promptstate.go` to filter monsters by location (similar to NPCs)
- Update `pkg/prompts/promptstate.go` `ToString()` method to include monsters in prompt output
- Add LLM guidance to system prompt builder

**Prompt Format**: Monsters should appear in a `MONSTERS:` section similar to the `NPCs:` section:

```
MONSTERS:
Giant Rat (AC: 12, HP: 9/9): A filthy, red-eyed rodent the size of a dog.

Giant Rat (AC: 12, HP: 7/9): A filthy, red-eyed rodent the size of a dog.
```

Include monster name, AC, HP (current/max), and description. Filter to only show monsters at the player's current location.

---

## API & Schema Updates (OpenAPI)

### New Endpoints

**GET /v1/monsters**
- Returns a list of available monster templates
- Response: `{"monsters": {"Giant Rat": "giant_rat", "Skeleton": "skeleton"}}`

**GET /v1/monsters/{templateID}**
- Returns a specific monster template
- Response: Monster JSON template

### Schema Updates

- **`Monster` schema** (template definition):
  - `id`, `name`, `description`, `ac`, `hp`, `max_hp`, `attributes{}`, `combat_modifiers{}`, `items[]`
  - `drop_items_on_defeat` (bool)
  - **Note**: Template does NOT include `location` (set at spawn time)

- **`MonsterInstance` schema** (in GameState):
  - Same as Monster template, plus `location` (current location of this instance)

- **`GameState`**:
  - Add `monsters` map (key: instance ID, value: MonsterInstance object)

- **`Location`**:
  - Add `monsters` map (key: instance ID, value: template ID)

- **`Scene.conditionals.then`**:
  - Add `spawn` array of objects: `{instance_id, template, location}`
  - Add `despawn` array of strings (instance IDs)

---

## Implementation Plan (Checklist)

**Step 1 — Actor Layer**
- [x] Create `pkg/actor/monster.go`
- [x] Implement struct, `NewMonster`, `TakeDamage`, `Heal`, `IsDefeated`, `MoveTo`

**Step 2 — Storage Layer**
- [x] Create `data/monsters/` directory for external monster JSON templates
- [x] Add `GetMonster(ctx, templateID)` method to Storage interface and RedisStorage implementation
- [x] Add `ListMonsters(ctx)` method to Storage interface and RedisStorage implementation
- [x] Create sample monster templates (e.g., `giant_rat.json`, `skeleton.json`)

**Step 3 — State Layer**
- [x] Extend `GameState` with `Monsters` map
- [x] Implement `SpawnMonster`, `DespawnMonster`, `EvaluateDefeats`
- [x] Ensure reducer calls `EvaluateDefeats()` each turn

**Step 4 — Scenario Loading**
- [x] Update `pkg/scenario/location.go` to include `Monsters map[string]string` field
- [x] In `internal/handlers/gamestate.go`, after initializing NPCs and locations, iterate through `WorldLocations`
- [x] For each location's `monsters` map, load templates and spawn instances using `storage.GetMonster()` and `gs.SpawnMonster()`

**Step 5 — Conditional Reducer (DeltaWorker)**
- [x] Extend conditional `then` actions to handle `spawn` and `despawn`
- [x] For spawn: Load template via storage, call `gs.SpawnMonster(instanceID, template, location)`
- [x] For despawn: Call `gs.DespawnMonster(instanceID)`

**Step 6 — Prompt Builder**
- [x] Update `pkg/prompts/promptstate.go` to include `Monsters` field
- [x] Filter monsters by location in `ToPromptState()` (similar to NPCs)
- [x] Update `ToString()` method to include MONSTERS section
- [x] Add narrator guidance not to invent monsters

**Step 7 — API Handlers**
- [x] Create `internal/handlers/monsters.go` with `GetMonsters` and `GetMonster` handlers
- [x] Wire up routes in API server

**Step 8 — OpenAPI**
- [x] Add Monster template schema to `/docs/openapi.yaml`
- [x] Update Location schema with `monsters` map
- [x] Add `/v1/monsters` and `/v1/monsters/{id}` endpoints
- [ ] Update conditionals schema with `spawn`/`despawn` actions (conditionals not yet in OpenAPI spec)

---

## Unit Tests

Location suggestions:
- `pkg/actor/monster_test.go`
- `pkg/state/gamestate_monster_test.go`
- `internal/reducer/conditionals_monster_test.go`

**Actor tests**
- [ ] `NewMonster` sets HP from MaxHP; clamps values ≥ 0
- [ ] `TakeDamage` reduces HP; cannot go below 0
- [ ] `Heal` cannot exceed `MaxHP`
- [ ] `IsDefeated` true when `HP <= 0`
- [ ] `MoveTo` updates location

**State tests**
- [ ] `SpawnMonster` inserts record, sets active
- [ ] `DespawnMonster` removes, drops items to location
- [ ] `EvaluateDefeats` auto-despawns defeated monsters

**Reducer/conditionals tests**
- [ ] `then.spawn` creates monsters in specified locations
- [ ] `then.despawn` removes monsters from game state
- [ ] Idempotency: repeated `then.spawn` for same ID no-ops or generates unique IDs (decide policy)

**Prompt builder tests**
- [ ] Monsters at player's location appear in prompt context; monsters at other locations do not

**OpenAPI validation tests (if applicable)**
- [ ] Updated spec compiles; required fields validated

---

## Integration Tests

Location suggestions:
- `internal/handlers/chat_monster_integration_test.go`

**End-to-end scenario flow**
1. Load scenario with a pre-placed monster → verify:  
   - Monster appears in `GameState`, correct location, narrator mentions presence
2. Trigger conditional `then.spawn` → verify:  
   - Monster spawns in correct location, narrator story event appears (if present)
3. Apply damage → verify:  
   - `EvaluateDefeats` despawns when `HP <= 0`
   - Items transfer to location (when enabled)
4. Ensure narrator does **not** invent monsters → verify prompts contain the explicit guidance and test with adversarial user inputs

**Persistence**
- Save/restore `GameState` (Redis/filesystem) preserves active monsters and locations

**API Contract**
- `GET /v1/gamestate/{id}` includes `monsters` map
- Scenario load accepts monsters in JSON and initializes correctly

---

## Examples

**Monster template (external JSON in `data/monsters/dire_wolf.json`):**
```json
{
  "id": "dire_wolf",
  "name": "Dire Wolf",
  "description": "A massive wolf with matted fur and ember eyes.",
  "ac": 13,
  "hp": 37,
  "max_hp": 37,
  "attributes": {
    "strength": 17,
    "dexterity": 15,
    "constitution": 13,
    "intelligence": 3,
    "wisdom": 12,
    "charisma": 6
  },
  "combat_modifiers": {
    "bite": 5
  },
  "items": ["fang", "pelt"],
  "drop_items_on_defeat": true
}
```

**Pre-placed monsters in scenario locations:**
```json
"locations": {
  "forest_path": {
    "name": "Forest Path",
    "description": "A dark trail through twisted trees.",
    "exits": {
      "north": "village"
    },
    "monsters": {
      "wolf_alpha": "dire_wolf",
      "wolf_beta": "dire_wolf"
    }
  }
}
```

**Conditional spawn/despawn:**
```json
"conditionals": {
  "wolves_appear": {
    "when": { "vars": { "night_falls": "true" } },
    "then": {
      "spawn": [
        {"instance_id": "night_wolf_1", "template": "dire_wolf", "location": "forest_path"},
        {"instance_id": "night_wolf_2", "template": "dire_wolf", "location": "forest_path"}
      ],
      "prompt": "STORY EVENT: Two dire wolves emerge from the darkness, eyes glowing."
    }
  },
  "wolves_retreat": {
    "when": { "vars": { "dawn_broke": "true" } },
    "then": {
      "despawn": ["night_wolf_1", "night_wolf_2"],
      "prompt": "STORY EVENT: The wolves slink back into the forest."
    }
  }
}
```

