---
name: story-engine-scenario
description: "Create, review, or expand story-engine scenario JSON files. Use when: creating a new scenario from scratch, creating a starter scenario, building a scenario outline, reviewing a scenario for correctness, adding scenes to an existing scenario, wiring the opening scene, adding global locations or NPCs, auditing top-level field order, setting narrator_id or default_pc. Delegates to sub-skills for locations, maps, NPCs, PCs, narrators, vars, and transitions. Trigger words: scenario, new scenario, starter scenario, create scenario, opening scene, opening location, opening prompt, story, rating, story-engine-scenario."
argument-hint: "starter <concept> | create <concept> | review | expand <aspect>"
---

# story-engine-scenario

Create and refine scenario JSON files for the story-engine. Produces complete, playable scenarios with correct field order, wired scenes, grounded locations, and clear narrative direction. Delegates specialized work to sub-skills.

## When to Use

- **starter**: Generate a minimal but playable scenario shell — one scene, one location, no inventory, empty rules
- **create**: Draft a full multi-scene scenario from a concept
- **review**: Audit an existing scenario against all quality criteria below
- **expand**: Add scenes, locations, NPCs, or transitions to an existing scenario

---

## Sub-Skill Delegation

This skill orchestrates the scenario. For specialized content, load and follow the appropriate sub-skill:

| Task | Sub-Skill |
|------|-----------|
| Writing or improving location descriptions | `story-engine-location` |
| Building a connected map of locations | `story-engine-map` |
| Writing or improving NPC definitions | `story-engine-npc` |
| Writing or reviewing a narrator | `story-engine-narrator` |
| Creating or reviewing a PC | `story-engine-pc` |
| Writing scene transitions (conditionals, vars) | `story-engine-transition` |
| Auditing or tuning vars and contingency prompts | `story-engine-vars` |

---

## Standard Catalog

### Available Narrators (`narrator_id`)

| ID | Voice |
|----|-------|
| `classic` | Traditional omniscient adventure narrator |
| `vincent_price` | Dramatic, theatrical Gothic horror |
| `noir` | Cynical, hard-boiled detective |
| `comedic` | Lighthearted and humorous |
| `christie` | Genteel, socially observant mystery |
| `poe` | Brooding, psychological horror |
| `tolkien` | Archaic, grandly literary fantasy |
| `howard` | Pulp action, visceral and muscular |

**When no narrator fits**: consider creating one with the `story-engine-narrator` skill.

### Available PCs (`default_pc`)

| ID | Concept |
|----|---------|
| `classic` | Generic adventurer — works in any setting |
| `alexandra_kane` | 1920s archaeologist / adventurer |
| `elara_wyn` | Elven ranger |
| `korga_ironblood` | Dwarf warrior |
| `thalion_silverwind` | High elf wizard |
| `owen_delaney` | Modern era everyman |
| `pirate_captain` | Caribbean pirate captain |
| `van_helsing` | Monster hunter |

**Choosing a PC**: Match genre and tone. `classic` is safe for novel settings. For horror use `van_helsing`; for fantasy use `elara_wyn`, `korga_ironblood`, or `thalion_silverwind`; for pulp adventure use `alexandra_kane`.

---

## Top-Level Field Order

Always emit fields in this order. It matches the schema and keeps files readable:

```
name
story
rating
temperature          (optional — omit if using 0.6 default)
narrator_id
default_pc
opening_scene
opening_location
opening_prompt
opening_inventory    (optional — omit if no starting items; never emit an empty array)
locations            (global location definitions)
npcs                 (global NPC definitions — {} if none)
scenes
contingency_prompts
contingency_rules
```

---

## Starter Scenario Procedure

A **starter** is the minimum viable scenario: one scene, one location, no inventory, empty rules arrays. Use it as a scaffolding to build on.

### Step 1 — Gather the concept

Before writing, identify:
- **Setting**: time period, world, genre
- **Premise** (one sentence): what is the player doing and why?
- **Opening mood**: the first impression the narrator should create
- **Narrator**: pick from the catalog above (or note that a new one is needed)
- **PC**: pick from the catalog above (or note that a new one is needed)

### Step 2 — Name the opening location

Choose a `snake_case` key for the opening location (e.g., `village_square`, `harbor_docks`, `dungeon_entrance`). This becomes `opening_location`.

### Step 3 — Name the opening scene

Choose a `snake_case` key for the opening scene (e.g., `arrival`, `opening`, `prologue`). This becomes `opening_scene`.

### Step 4 — Write the opening location

Apply the `story-engine-location` skill standards:
- 2–4 sentences, spatially complete
- Exits described in prose AND in the `exits` map
- No NPCs, plot events, or dynamic content in `description`
- Add a `preview` (one sentence) for adjacent-location context

### Step 5 — Write the opening prompt

The `opening_prompt` is the narrator speaking directly to the player at the start of the game. Write it in second person ("You are…", "You stand…"). This is the one field where second-person is correct. Aim for 2–4 sentences.

### Step 6 — Write the opening scene

The opening scene needs only a `story` field for the starter. One or two sentences describing what this scene is about — think of it as the director's note for the narrator.

### Step 7 — Assemble the starter JSON

Use this exact structure and field order:

```json
{
  "name": "Your Scenario Title",
  "story": "One or two sentence premise from the player's perspective.",
  "rating": "PG-13",
  "narrator_id": "classic",

  "default_pc": "classic",
  "opening_scene": "opening_scene_key",
  "opening_location": "opening_location_key",
  "opening_prompt": "You stand at the threshold of... (narrator voice, 2nd person, 2-4 sentences)",
  "locations": {
    "opening_location_key": {
      "name": "Display Name",
      "description": "Spatially complete description. 2-4 sentences. Exits described in prose.",
      "preview": "One-sentence summary for adjacent location display.",
      "exits": {
        "direction": "other_location_key"
      }
    }
  },
  "npcs": {},
  "scenes": {
    "opening_scene_key": {
      "story": "Director note: what this scene is about."
    }
  },
  "contingency_prompts": [],
  "contingency_rules": []
}
```

**Starter rules:**
- `rating` — default to `"PG-13"` unless concept clearly warrants something else
- `opening_inventory` — omit entirely; never emit `"opening_inventory": []`
- `locations` — global level only; no scene-level locations in the starter
- `npcs` — empty object `{}`, not an array
- `contingency_prompts` and `contingency_rules` — empty arrays `[]`
- `temperature` — omit (use system default 0.6)
- `game_end_prompt` — omit

---

## Full Scenario Procedure

For a complete multi-scene scenario with NPCs, transitions, and game mechanics.

### Step 1 — Story outline

Before writing any JSON, pin down:
- **Premise**: one paragraph
- **Scene list**: what scenes exist, in what order?
- **Transition conditions**: what must the player do to move from scene to scene?
- **Key NPCs**: who drives the story?
- **Map shape**: how many locations, how are they connected?

### Step 2 — Build the map

Use the `story-engine-map` skill to design connected locations. Produce a full `locations` block with correct exits, previews, and descriptions. Place the map at the global level unless locations are scene-exclusive.

### Step 3 — Define global NPCs

Use the `story-engine-npc` skill for each NPC. Inline NPCs belong directly in the `npcs` map. Use `template_id` for NPCs that need combat stats or appear in multiple scenarios.

### Step 4 — Write scenes

Each scene needs:
- `story` — the narrator/director brief for this scene
- `vars` — any boolean flags this scene introduces (initialize to `"false"`)
- `contingency_prompts` — narrative hints for the LLM (soft guidance)
- `contingency_rules` — state-change instructions for the LLM (what to set when)
- `conditionals` — deterministic enforcement of scene transitions and story events

For transitions between scenes: load and follow the `story-engine-transition` skill.
For vars and contingency prompts: load and follow the `story-engine-vars` skill.

### Step 5 — Write the opening prompt and inventory

- `opening_prompt`: second-person, narrator voice, 2–4 sentences setting the scene
- `opening_inventory`: list starting items as strings, or omit if none

### Step 6 — Assemble

Emit all top-level fields in the order defined in **Top-Level Field Order** above.

---

## Field Reference

### `name`
Display title of the scenario. Use title case. Keep it evocative, not descriptive.

### `story`
1–3 sentence premise written from the **player's third-person perspective** ("The player is a..."). This is the narrator's background context, not the opening prompt. Do not begin with "You."

### `rating`
Content rating string. Use the canonical values from code:

| Value | Constant | Meaning |
|-------|----------|---------|
| `"G"` | `RatingG` | Suitable for all ages |
| `"PG"` | `RatingPG` | Parental guidance suggested |
| `"PG-13"` | `RatingPG13` | Parents strongly cautioned |
| `"R"` | `RatingR` | Restricted to adults |

Default to `"PG-13"` when the tone involves peril, violence, or mature themes but nothing explicitly adult. Use `"PG"` for younger-friendly adventures and `"R"` only when the scenario is intentionally adult.

### `temperature`
Float 0.0–1.0. Omit to use system default (0.6). Lower = more predictable; higher = more creative. Can be overridden per scene.

### `narrator_id`
String ID from `data/narrators/`. See the Standard Catalog above. Must match an existing file (without `.json`).

### `default_pc`
String ID from `data/pcs/`. See the Standard Catalog above. Must match an existing file (without `.json`). Use `"classic"` when no specific character is needed.

### `opening_scene`
`snake_case` key of the scene to load first. Must match a key in `scenes`.

### `opening_location`
`snake_case` key of the location where the player starts. Must match a key in `locations` (global or inside the opening scene).

### `opening_prompt`
The narrator's first words to the player. Second-person voice ("You…"). 2–4 sentences. Sets tone and grounds the player in the world without over-explaining the plot.

### `opening_inventory`
Array of strings. Items the player starts with. **Omit entirely when there is no starting inventory — never emit an empty array.**

### `locations`
Map of `snake_case` location keys → location objects. Global locations are accessible across all scenes. Scenes can add their own locations which are only active during that scene.

See `story-engine-location` skill for location quality standards.

### `npcs`
Map of `snake_case` NPC keys → NPC objects. Global NPCs persist across all scenes. Use `{}` for no global NPCs.

See `story-engine-npc` skill for NPC quality standards.

### `scenes`
Map of `snake_case` scene keys → scene objects. At minimum, the scene referenced by `opening_scene` must be present.

### `contingency_prompts` (top-level)
Array of strings or conditional objects. Scenario-wide narrative hints for the LLM, active throughout all scenes. Use sparingly — prefer scene-level prompts for scene-specific guidance.

### `contingency_rules` (top-level)
Array of strings. Scenario-wide state-change instructions for the LLM. Active throughout all scenes.

---

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| `opening_location` key not present in `locations` | Add the location to the top-level `locations` map |
| `opening_scene` key not present in `scenes` | Add the scene to `scenes` |
| `narrator_id` or `default_pc` does not match a file | Check `data/narrators/` and `data/pcs/` for valid IDs |
| Second-person in `story` field | `story` is third-person context; only `opening_prompt` uses second-person |
| NPCs defined as `[]` instead of `{}` | NPCs is a map (object), not an array |
| Location exits exist in the map but not in prose | Describe every exit in the location `description` |
| Scene-level locations in the starter | Starter locations go in the global `locations` block only |
| `opening_inventory` included as empty array | Omit the field entirely; never emit `"opening_inventory": []` |
| `game_end_prompt` included in starter | Omit it; only add when the scenario has a defined end condition |

---

## Quality Checklist

### Starter scenario

- [ ] `name`, `story`, `rating`, `narrator_id`, `default_pc` all present
- [ ] `opening_scene` matches a key in `scenes`
- [ ] `opening_location` matches a key in `locations`
- [ ] `opening_prompt` is second-person, 2–4 sentences
- [ ] Opening location description is spatially complete (2–4 sentences)
- [ ] Every exit in `exits` map is described in prose
- [ ] `npcs` is `{}` (object, not array)
- [ ] `contingency_prompts` and `contingency_rules` are `[]`
- [ ] `opening_inventory` omitted (not an empty array)
- [ ] `temperature` omitted
- [ ] Fields in correct order (name → story → rating → narrator_id → default_pc → opening_scene → opening_location → opening_prompt → locations → npcs → scenes → contingency_prompts → contingency_rules)

### Full scenario (additional checks)

- [ ] All scenes referenced by `scene_change` exist in `scenes`
- [ ] All locations referenced by NPCs exist in `locations` (global or scene-level)
- [ ] All vars used in `when` clauses are initialized in scene `vars` blocks
- [ ] Every scene transition has a var + conditional (not just a contingency rule)
- [ ] No story-specific events in location `description` fields
- [ ] Contingency prompts count is reasonable (≤ 6 per scene; prefer 2–4)
