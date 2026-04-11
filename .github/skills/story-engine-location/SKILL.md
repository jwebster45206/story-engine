---
name: story-engine-location
description: "Write, review, or improve location definitions in story-engine scenario JSON files. Use when: creating a new location, improving a thin or vague description, adding sensory detail, auditing exits, reviewing locations for quality, or fixing a description that leaks dynamic plot elements. Trigger words: location, description, exits, sensory, atmosphere, scenario, scene, chamber, room, story-engine-location."
argument-hint: "create <name> | review | add-sensory | audit-exits"
---

# story-engine-location

Write and refine location definitions for story-engine scenario JSON files. Produces locations with complete spatial descriptions, sensory grounding, clearly stated exits, and no dynamic or plot-contingent content in the `description` field.

## When to Use

- **Create**: Draft a new location from a concept, name, or story context
- **Review**: Score existing locations against all quality criteria below
- **Add sensory detail**: Expand a thin description with sight, sound, and smell
- **Audit exits**: Verify exits are described in prose and match the `exits` map

---

## Location Struct Reference

```go
type Location struct {
    Name               string
    Description        string                    // narrator-visible; always present in prompts
    Preview            string                    // short summary for adjacent-location prompts
    Exits              map[string]string         // direction → locationKey
    BlockedExits       map[string]string         // direction → reason why blocked
    Items              []string                  // items present here
    Monsters           map[string]*actor.Monster // pre-placed monsters
    IsImportant        bool                      // if true, always included in gamestate
    ContingencyPrompts []ContingencyPrompt       // injected only while player is here
}
```

`Description` is injected into every narrator prompt while the player is at this location. It is permanent and static — it cannot vary based on story state.

`Preview` is shown to the narrator for **adjacent/nearby** locations — never the current one. If omitted, only the location name appears. This prevents description bleed across the map.

---

## Core Principles

### 1. Description Must Be Spatially Complete

The player and narrator should be able to orient themselves from the description alone. Every location needs:

- **Size** — approximate is fine: "a cramped alcove", "a vast cathedral nave", "a narrow corridor"
- **Shape and dominant features** — what would you see standing at the entrance?
- **Materials and texture** — stone, wood, mud, silk; rough, smooth, rotting, gleaming
- **Light quality** — torchlit, moonlit, pitch dark, overcast, blinding

A single-sentence description is almost always incomplete. Aim for 2–4 sentences minimum.

| Too thin (reject) | Spatially complete (accept) |
|---|---|
| "A busy port town." | "A cramped harbor district where salt-warped buildings lean over cobblestones slick with fish guts. Dock rigging creaks in the wind and the smell of tar and brine hangs in the damp air." |
| "The captain's cabin." | "A low-ceilinged oak cabin spanning the full width of the stern. Charts and instruments cover a bolted-down table; a hammock sways in the far corner. Candlelight throws long shadows across the carved portraits on the walls." |
| "A dungeon cell." | "A stone cell barely wide enough to stretch your arms, with a ceiling so low the air smells of damp and rot. A rusted grate in the floor drains black water. The only light filters through a finger-wide slit high in the outer wall." |

### 2. Include Other Senses When They Add Meaning

Sight is the default; add sound, smell, or temperature when those senses are prominent or striking. Don't force all senses into every location — choose the ones that define the space.

| Sense | Use when... |
|---|---|
| Sound | Echoes, silence, ambient noise, machinery, nature |
| Smell | Strongly atmospheric (sea air, rot, incense, smoke, sulfur) |
| Temperature | Extreme cold/heat, a contrast to the previous location |
| Texture | The player is likely to touch something (walls, floors, objects) |

### 3. Exits Must Appear in Prose

Every exit in the `exits` map (and every notable `blocked_exit`) must be described in the `description` field. The narrator needs to be able to mention exits naturally without referring to a data structure.

**Formats that work:**
- `"To the north, the passage continues into darkness."`
- `"A stairway rises through a square opening in the vaulted ceiling."`
- `"Three archways open in the far wall; the western one is bricked up to shoulder height."`
- `"The only door — iron-banded oak — stands at the south end."`

**Anti-pattern:** Exits exist in the JSON but are invisible in the description. The narrator is left with no spatial language to guide the player.

### 4. No Dynamic or Plot-Contingent Content

`description` is injected on **every** turn the player is at this location, regardless of story state. Never include:

- References to specific NPCs being present (`"The innkeeper leans on the bar."`)
- Plot-specific conditions (`"A body lies on the floor."` — unless it is a permanent fixture of the location)
- Items that change as the story progresses
- Combat or threat status

**Where dynamic content belongs instead:**

| Content type | Correct field |
|---|---|
| NPC presence at this location | NPC's `location` field + narrator contingency prompts |
| Plot-specific conditions | `contingency_prompts` with a `when` guard |
| One-time discoveries | Scene prompts or scenario-level `contingency_prompts` |
| Ambient threats (always present) | Description is fine — "Rats skitter along the walls" |

**The test:** Could this description appear at the start of the game, mid-game, and after all major plot events without being false or misleading? If no, move the problematic content to `contingency_prompts`.

### 5. NPCs: Be Careful

An NPC standing in a specific location is dynamic state — they can move, die, or leave. Do not embed specific NPC presence in `description`.

**Acceptable:** General evidence of habitation with no named occupant.
- `"A clerk's stool stands before a tall ledger desk."` — implies a clerk without committing to one being there.

**Not acceptable:**
- `"Old Marta sits by the fire."` — Marta might be dead or gone.

---

## Review Checklist

Score each location against:

- [ ] **Spatial complete** — size, dominant features, materials, light described
- [ ] **Sensory depth** — at least sight; sound/smell added where meaningful
- [ ] **Exits in prose** — every `exits` key geographically described; `blocked_exits` noted
- [ ] **No dynamic content** — no NPC presence, no plot-state conditions
- [ ] **Narrator-neutral** — description doesn't commit to a narrator's voice or tone (the narrator adds that); plain but evocative language
- [ ] **Length** — 2–4 sentences minimum; longer for important or complex spaces
- [ ] **Preview present** — 1 sentence, spoiler-free summary for adjacent-location display

---

## Field-by-Field Standards

### `description`

- Write in present tense, second-person-neutral ("A corridor stretches north…" not "You see a corridor…")
- Plain, evocative language — the narrator will filter it through their voice
- Exits appear naturally in the final sentence or two
- No spoilers, no plot state, no named NPCs in situ

### `preview`

1 sentence. A short, spoiler-free summary shown to the narrator when this location is an adjacent/nearby location. It should convey the location's identity without leaking its full atmosphere, items, NPCs, or plot state. If omitted, only the location name is shown to the narrator.

| Too much (description leaking) | Good preview |
|---|---|
| `"A dank, dark cellar beneath the tavern. Barrels of rum and rotting crates line the walls. The scurrying of rats echoes in the darkness."` | `"The cellar beneath the Sleepy Mermaid."` |
| `"A bustling market filled with merchants, pirates, and rare goods. The air is thick with the scent of spices and the sound of haggling."` | `"An open-air market for goods and gossip."` |

### `exits`

- Keys are directions or descriptive navigation verbs: `"north"`, `"stairs up"`, `"cabin door"`, `"through the arch"`
- Values are location keys matching the scenario's `locations` map
- Every exit key should correspond to prose in `description`

### `blocked_exits`

- Keys match exits that *look* passable in the description
- Values are short reasons the narrator can paraphrase: `"Lots of British soldiers in northern docks."`
- Do not put a blocked exit here if it is also in `exits` — it will confuse the narrator

### `contingency_prompts`

- Use for dynamic conditions that apply only while the player is at this location
- Plain string format: `"The cellar smells of fresh blood — something died here recently."`
- Conditional format (use `when` guards) for state-dependent content:
  ```json
  {
    "prompt": "The forge is cold — the blacksmith has not worked today.",
    "when": { "vars": { "blacksmith_present": false } }
  }
  ```

### `items`

- List items that are meaningfully present and findable
- Do not duplicate items that are already in a specific NPC's inventory
- Avoid listing ambient objects (barrels of rum in a tavern cellar = description, not items, unless they are interactive)

### `important`

- Set `true` only for locations that are always narratively relevant regardless of where the player is
- Default: omit (treated as false)

---

## Examples

### Minimal but complete

```json
"forge": {
  "name": "The Blacksmith's Forge",
  "description": "A low stone building open on the south wall to the village square. An iron anvil the size of a tree stump dominates the center; coals smolder in the wide hearth to the east. The smell of hot metal and charcoal is constant. A narrow door in the north wall leads to a storage room.",
  "preview": "The village blacksmith's workshop.",
  "exits": {
    "south": "village_square",
    "north": "forge_storage"
  }
}
```

### With sensory layering

```json
"crypt_entrance": {
  "name": "Crypt Entrance",
  "description": "A vaulted antechamber carved from dark stone, roughly thirty feet across. Iron sconces hold half-burned candles that drip wax onto the flagstone floor. The air is cold and carries a faint sweetness that is not quite flowers. To the west, steep stairs descend into darkness; to the east, a heavy iron door stands open, its hinges green with age.",
  "preview": "The antechamber at the top of the crypt stairs.",
  "exits": {
    "stairs down": "crypt_lower",
    "east": "crypt_chapel"
  }
}
```

### With blocked exit noted in prose

```json
"village_north_road": {
  "name": "North Road",
  "description": "A rutted track that runs north out of the village before disappearing into a grey wall of fog. A barricade of lashed timber and farm carts has been dragged across the road fifty yards ahead. South, the village square is visible through the mist. A footpath branches east toward the mill.",
  "preview": "A muddy road heading north out of the village.",
  "exits": {
    "south": "village_square",
    "east": "mill_path"
  },
  "blocked_exits": {
    "north": "Barricade — villagers have blocked the road. They will not let anyone through."
  }
}
```
