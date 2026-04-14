---
name: story-engine-map
description: "Build a connected set of locations (a map) for a story-engine scenario from a user's description or concept. Use when: designing or expanding a scenario map, wiring exits between locations, adding blockages, auditing that a map is fully connected, inventing bridge locations to fill gaps, or populating the locations block of a scenario JSON file. Trigger words: map, locations, connected, exits, blockage, bridge location, scenario map, story-engine-map."
argument-hint: "build <concept> | expand <existing> | audit-connectivity | add-blockage"
---

# story-engine-map

Translate a user's map concept into a fully connected, error-free `locations` block in a scenario JSON file. Focuses on topology (exits, connections, blockages) and delegates individual location quality to the `story-engine-location` skill.

## When to Use

- **Build**: Draft a complete map from a concept, setting, or list of named areas
- **Expand**: Add new locations to an existing map without breaking existing exits
- **Audit connectivity**: Verify every location is reachable from the opening location
- **Add blockage**: Wire a blocked exit with a reason and prose mention
- **Fix broken exits**: Repair mismatched, one-sided, or missing exit keys

---

## Relationship to `story-engine-location`

This skill owns the **map topology**: which locations exist, how they connect, and what is blocked. It does not own the internal quality of individual location descriptions.

When writing descriptions, apply `story-engine-location` standards:
- Spatially complete (size, dominant features, materials, light)
- Exits appear in prose before they appear in the `exits` map
- No dynamic or plot-contingent content in `description`
- A one-sentence `preview` for every location

If a specific location needs deeper review or rework, invoke `story-engine-location` on it directly.

---

## Core Topology Rules

### 1. Every exit must have a return path

If location A has an exit to B, B must have an exit back to A — unless it is intentionally one-way (e.g., a trapdoor that drops and cannot be climbed). One-way exits are rare and should be flagged.

| Exit key style | Use for |
|---|---|
| `"north"` / `"south"` / `"east"` / `"west"` | Open terrain, simple grids |
| `"up"` / `"down"` | Stairs, ladders, wells, cliffs |
| Descriptive: `"cabin door"`, `"stairs up"`, `"through the arch"` | Named portals, ships, buildings |

Return exits should use the natural inverse: `"north"` ↔ `"south"`, `"up"` ↔ `"down"`, `"cabin door"` ↔ `"cabin door"`.

### 2. Every location must be reachable

Starting from the `opening_location`, there must be a traversable path (ignoring `blocked_exits`) to every location in the map. If not, invent a **bridge location** to connect the gap.

### 3. Blocked exits are exits that look passable

A `blocked_exit` is a direction the player can see but cannot currently use. It must:
1. Appear in the location's `description` prose — the narrator needs language to mention it
2. Appear in `blocked_exits` with a short reason the narrator can paraphrase
3. **Not** also appear in `exits` — a direction cannot be both open and blocked

### 4. Location keys are lowercase snake_case

Keys like `"black_pearl"`, `"captains_cabin"` are internal IDs used in exits, NPC locations, and game state. The `"name"` field is the display string. Never use spaces or uppercase letters in a key.

### 5. Keep the map to a navigable size

A playable scenario map is typically 5–12 locations. More than 15 locations risks player confusion without strong wayfinding. When in doubt, combine adjacent thin locations into one richer one.

---

## Procedures

### 1. Build a Map from a Concept

1. **Extract named areas** from the user's prompt. Each distinct named area is a candidate location. Unnamed connective tissue ("the halls between them") may become a bridge location or a named corridor.

2. **Sketch the topology** before writing JSON. List locations and draw connections:
   ```
   tortuga ──east──► black_pearl
      │                   │
    south             cabin door
      │                   │
   sleepy_mermaid    captains_cabin
      │
   back door
      │
   tortuga_market
   ```

3. **Check connectivity**: Can you reach every location from the opening location? If a location is isolated, add a bridge or a direct connection.

4. **Identify blockages**: Did the user mention guarded, locked, or inaccessible routes? Add those as `blocked_exits` with reasons.

5. **Assign exit keys**: Choose direction words or descriptive portal names. Assign the reciprocal exit on the other side.

6. **Write each location** using `story-engine-location` standards:
   - `name`: Display string, any formatting
   - `description`: 2–4 sentences. Spatially complete. Exits appear in prose. No plot state.
   - `preview`: 1 sentence, spoiler-free summary for adjacent-location display
   - `exits`: map of exit key → location key
   - `blocked_exits`: map of direction → reason (if applicable)

7. **Assemble into the scenario JSON** under the top-level `"locations"` key. Maps always live at scenario level — see [Where Maps Live](#where-maps-live).

8. **Self-audit** using the checklist below before delivering.

### 2. Expand an Existing Map

1. Read the existing `locations` block.
2. Identify where the new location(s) attach. Find the existing location(s) they connect to.
3. Add the new location(s) and their exits.
4. Add the reciprocal exits on existing locations.
5. Verify connectivity is still intact.
6. Check that no existing exit keys are clobbered.

### 3. Audit Connectivity

1. Identify the `opening_location` key.
2. Build the reachability set: start from `opening_location`, follow all `exits` recursively.
3. Compare against all keys in `locations`. Any key not in the reachability set is **isolated**.
4. For isolated locations: propose a bridge connection or flag it for the user to decide.

### 4. Add a Blockage

1. Identify the direction and the location it would lead to.
2. Confirm the direction appears (or should appear) in the location's `description` prose. If not, update the description to mention it.
3. Add the direction to `blocked_exits` with a concise reason.
4. Confirm the direction is **not** also in `exits`.
5. On the destination side: the location still needs a return exit listed in `exits` for when the blockage is lifted — or note that it will be added in a scene override when the blockage clears.

### 5. Design Bridge Locations

When two areas need a connection but no logical direct path exists, invent a short bridge location:

- **Keep it thin but functional**: a corridor, a stairway, a dock, a crossroads
- **Name it descriptively**: `"market_alley"`, `"east_stairwell"`, `"harbor_path"`
- **The description earns its place**: even utilitarian connectors need spatial grounding — materials, light, what can be seen from here
- **Exits point both ways**: the bridge connects A to B and B to A
- Flag invented bridge locations explicitly when presenting to the user, so they can rename or remove them

---

## Exit Key Conventions

| Situation | Preferred key style |
|---|---|
| Simple grid (overworld, dungeon) | `"north"`, `"south"`, `"east"`, `"west"` |
| Vertical movement | `"up"` / `"down"`, `"stairs up"` / `"stairs down"` |
| Named portal (ship, building) | `"cabin door"`, `"through the arch"`, `"storm drain"` |
| Named building as a location | `"sleepy mermaid back door"`, `"gate"` |
| Mixed (overworld + building) | Use descriptive keys for building entries so they read naturally in prose |

**Do not mix compass and descriptive keys arbitrarily.** Pick a dominant convention for the scenario and be consistent. Compass + named portals for special transitions is a clean pattern.

---

## Where Maps Live

**Always write maps at the top-level `"locations"` key in the scenario JSON.** This is the canonical world geography for the entire scenario.

```json
{
  "name": "My Scenario",
  "locations": {
    "village_square": { ... },
    "inn": { ... }
  }
}
```

Scene-level location overrides (inside `scenarios.scenes[].locations`) can selectively modify a location's exits or description for a specific story phase — but that is the concern of `story-engine-transition`, not this skill. Build the full map at scenario level first.

---

## Map Checklist

Before delivering any map:

- [ ] **Every exit has a reciprocal** — both sides of each connection are wired
- [ ] **All locations reachable** — starting from `opening_location`, every location is traversable
- [ ] **Blocked exits in prose** — every `blocked_exits` direction is described in `description`
- [ ] **No key in both `exits` and `blocked_exits`** — a direction cannot be both open and blocked
- [ ] **All location keys are lowercase snake_case** — no spaces, no uppercase
- [ ] **Exit values match location keys** — every exit target exists as a key in `locations`
- [ ] **Each location has `name`, `description`, `preview`** — no bare minimal stubs
- [ ] **Descriptions are spatially complete** — size, dominant features, materials, light
- [ ] **No dynamic content in `description`** — no NPCs in situ, no plot state
- [ ] **Map size is navigable** — 5–12 locations for a standard scenario

---

## Example: Rural Village with Interior Sub-Map

This example demonstrates all three exit types — compass navigation for the open world, object-based exits for building entry and interior movement — plus a blocked exit and a looping external map.

### Topology sketch

Always draw this before writing JSON. It surfaces missing reciprocals and broken loops before you touch a bracket.

```
                      [church]
                          |
                       S / N
                          |
[west_road] ──east──  [village_square] ──east──  [mill] ──cottage door──  [millers_cottage]
     |                      |                       |                              |
 pond path              south                   north                      up the ladder
     |                      |                       |                              |
[millpond] ←── same node ──────────────────── [millpond]                   [millers_loft]
  (loop: village_square → W → west_road → pond path → millpond → S → mill → W → village_square)

  [church] ──east── [churchyard]  (blocked: N — crypt gate rusted shut)
  [village_square] ──south── [inn] ──up the stairs── [inn_upstairs]
```

**The loop:** `village_square → W → west_road → pond path → millpond → S → mill → W → village_square`

**Interior sub-map:** `mill → cottage door → millers_cottage → up the ladder → millers_loft`

**Spurs:** `church → churchyard` (dead end with blocked exit); `inn → inn_upstairs`

---

### JSON

```json
"locations": {

  "village_square": {
    "name": "Village Square",
    "description": "A broad dirt square at the heart of the village, ringed by low stone buildings. A dry well stands at the center. The church rises to the north, its weathervane motionless against a grey sky. To the east, the mill's wheel turns slowly over the stream; to the south, lantern light spills from the inn. A rutted track heads west out of the square.",
    "preview": "The central square of the village.",
    "exits": {
      "north": "church",
      "east": "mill",
      "south": "inn",
      "west": "west_road"
    }
  },

  "church": {
    "name": "St. Aldric's Church",
    "description": "A squat Norman church of dark limestone, its nave barely wider than a barn. Tallow candles burn in iron holders along the walls, and the air smells of cold wax and old stone. A narrow door in the east wall leads to the churchyard. The village square lies to the south.",
    "preview": "A small stone church at the north end of the village.",
    "exits": {
      "south": "village_square",
      "east": "churchyard"
    }
  },

  "churchyard": {
    "name": "Churchyard",
    "description": "Mossy headstones lean at odd angles among the long grass, their inscriptions worn smooth by decades of rain. A low wall borders the yard on three sides. To the north, a heavy iron gate in the far wall leads to the old crypt path — but the gate is rusted solid and will not move. The church door is to the west.",
    "preview": "The overgrown graveyard beside the church.",
    "exits": {
      "west": "church"
    },
    "blocked_exits": {
      "north": "The crypt gate is rusted shut and will not budge."
    }
  },

  "inn": {
    "name": "The Plough Inn",
    "description": "A low-ceilinged common room with smoke-blackened beams and a hearth broad enough to stand in. Rough tables and benches fill most of the floor. A steep staircase in the far corner leads to the rooms above. The village square is through the door to the north.",
    "preview": "A modest inn on the south side of the square.",
    "exits": {
      "north": "village_square",
      "up": "inn_upstairs"
    }
  },

  "inn_upstairs": {
    "name": "Inn — Upper Landing",
    "description": "A narrow landing at the top of the stairs, with two low doors leading to simple guest rooms. The floorboards creak underfoot and a single tallow candle gutters in a wall sconce. The stairs lead back down to the common room.",
    "preview": "The upper floor of the Plough Inn.",
    "exits": {
      "down": "inn"
    }
  },

  "west_road": {
    "name": "West Road",
    "description": "A rutted cart track that curves northwest out of the village before bending toward the millpond. Hedgerows press in on both sides, their branches meeting overhead in a low tunnel of green. The village square is visible to the east; the pond path leaves the road and winds north toward the water.",
    "preview": "A dirt road leading west and north from the village.",
    "exits": {
      "east": "village_square",
      "pond path": "millpond"
    }
  },

  "millpond": {
    "name": "Millpond",
    "description": "A dark oval of still water fed by a narrow stream from the hills. Reeds crowd the near bank and the surface is dusted with pollen. The mill building is visible to the south, its wheel audible from here. The pond path winds southwest through the hedgerows back toward the west road.",
    "preview": "A quiet millpond north of the village.",
    "exits": {
      "south": "mill",
      "pond path": "west_road"
    }
  },

  "mill": {
    "name": "The Mill",
    "description": "A two-storey stone building straddling the stream, its great wheel turning with a steady creak and splash. Grain dust coats every surface inside and the air smells of fresh flour. A low cottage door in the north wall leads to the miller's home. The village square is to the west, and the millpond path heads north.",
    "preview": "A working watermill on the east edge of the village.",
    "exits": {
      "west": "village_square",
      "north": "millpond",
      "cottage door": "millers_cottage"
    }
  },

  "millers_cottage": {
    "name": "Miller's Cottage — Main Room",
    "description": "A single snug room with a flagstone floor and a small iron stove in the corner. A workbench along one wall holds tools and leather scraps; a rag rug covers the center of the floor. A wooden ladder is fixed to the far wall, leading up to the loft. The cottage door opens back into the mill.",
    "preview": "The ground floor of the miller's cottage.",
    "exits": {
      "cottage door": "mill",
      "up": "millers_loft"
    }
  },

  "millers_loft": {
    "name": "Miller's Cottage — Loft",
    "description": "A low-ceilinged loft just tall enough to stand in at the center. Straw is piled along the eaves and a small window looks out over the millpond. A rope-and-plank ladder drops back down to the main room below.",
    "preview": "The sleeping loft above the miller's cottage.",
    "exits": {
      "down": "millers_cottage"
    }
  }

}
```

### Topology verified

| Check | Result |
|---|---|
| All 10 locations reachable from `village_square` | ✓ |
| Loop intact | `village_square → W → west_road → pond path → millpond → S → mill → W → village_square` ✓ |
| All exits bidirectional | ✓ (portal keys use the same string on both ends; vertical keys use natural inverse pairs: `up` ↔ `down`) |
| Blocked exit in prose | `churchyard` north described in description ✓ |
| `north` not in both `exits` and `blocked_exits` | ✓ |
| All keys lowercase snake_case | ✓ |
| All exit targets exist as location keys | ✓ |

---

## Author Tips for Keeping a Map Straight

**Draw the topology sketch first.** JSON is a terrible medium for spotting a missing reciprocal or a broken loop. A quick ASCII diagram (like the one above) takes two minutes and prevents hours of debugging.

**Use compass keys for the exterior, object keys for interiors.** Compass exits (`north`, `south`) feel natural outdoors. Once the player crosses a threshold into a building, switch to descriptive keys (`cottage door`, `up the ladder`). Mixing them arbitrarily inside a single location is confusing.

**Descriptive exit keys are their own reciprocal.** If the exit into a building is `"cottage door": "millers_cottage"`, the exit back is also `"cottage door": "mill"`. Do not invent a different name for the return (`"exit"`, `"leave"`, `"back"`) — it breaks immersion and makes auditing harder.

**Prefix interior locations with the building name.** `millers_cottage` and `millers_loft` are instantly recognizable as part of the same sub-map. `room_1` and `loft` are not. This matters when you have multiple buildings.

**Every dead end should earn its place.** A location with only one exit (like `millers_loft`) is fine if it is a meaningful destination. If it is not — if the player has no reason to go there — merge it into its parent or give it a second connection.

**Blocked exits need prose support.** If the narrator cannot see the blocked direction in the `description`, they cannot mention it. The player will miss it entirely. Always write the blocked exit into the prose first, then add it to `blocked_exits`.

**Loops prevent dead ends in the external map.** A map where every path dead-ends creates a "backtrack everything" experience. One good loop (like the west_road/millpond/mill loop above) gives the player a sense of a real traversable world.
