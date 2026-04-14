---
name: story-engine-npc
description: "Write, review, or improve NPC definitions for the story-engine. Use when: creating a new NPC, choosing between inline vs sidecar NPC, improving a vague description, writing contingency_prompts, fixing story bleed in NPC fields, auditing NPC for narrator usability, or creating a standalone NPC JSON template. Trigger words: NPC, non-player character, sidecar, template_id, inline NPC, npc description, npc disposition, story-engine-npc."
argument-hint: "create <concept> | review | fix-story-bleed | tune-prompts | inline-vs-sidecar"
---

# story-engine-npc

Write and refine NPC definitions for the story-engine. Produces NPCs with specific dispositions, narrator-ready descriptions, behavior-anchored contingency prompts, and zero story bleed.

## When to Use

- **Create**: Draft a new NPC from a concept (role, personality, scenario purpose)
- **Review**: Score an existing NPC against all quality criteria below
- **Fix story bleed**: Remove plot-specific events from `description` or `contingency_prompts`
- **Tune prompts**: Sharpen contingency prompts from vague hints to specific behavioral cues
- **Inline vs sidecar**: Decide whether an NPC belongs as a sidecar file or inline definition

---

## NPC Struct Reference

### Fields the narrator sees in the world state prompt

```
Name (disposition) [AC: HP/MaxHP]: Description; Items: item1, item2
```

**Critical:** `type` is **not** injected into the narrator prompt. It is engine metadata only. Do not rely on it to communicate character information to the narrator.

`contingency_prompts` are injected as separate behavioral hints into the system prompt **only when the player is at the same location as the NPC**. They are never shown to the player.

### Full struct

```go
type NPC struct {
    TemplateID   string            // sidecar: points to data/npcs/{id}.json
    Name         string            // display name in prompts
    Type         string            // engine metadata only — not narrator-visible
    Disposition  string            // injected as "(disposition)" next to name
    Description  string            // appears after the name/disposition line
    IsImportant  bool              // if true, NPC appears in prompt even when not co-located
    Location     string            // current location key
    Following    string            // "pc" or NPC key — auto-synced by engine
    Items        []string          // shown as "; Items: ..." in prompts
    // Actor properties (sidecar only, optional)
    AC, HP, MaxHP  int
    Attributes     map[string]int  // {"strength": 14, ...}
    CombatMods     map[string]int  // {"longsword": 5}
    DropItemsOnDefeat bool
    ContingencyPrompts []ContingencyPrompt
}
```

---

## Inline vs Sidecar

### Use **inline** when:
- The NPC appears in exactly one scenario
- The NPC has no combat stats
- The character is specific to the story (a villain, a named ally, a bartender at this particular tavern)
- You want the full NPC definition visible in the scenario file for easy editing

### Use **sidecar** (`data/npcs/{id}.json` + `template_id`) when:
- The NPC may appear across multiple scenarios
- The NPC needs **actor properties** (HP, AC, combat stats)
- The character is a generic archetype that can be reused (guard captain, innkeeper, merchant)
- You want to keep the scenario file lean

### Sidecar override rules

In the scenario's `npcs` map entry, set `template_id` and provide only the fields that differ from the template. All other fields come from the file.

```json
"npcs": {
  "captain": {
    "template_id": "guard_captain",
    "location": "city_gate",
    "disposition": "suspicious"
  }
}
```

**Overridable**: all string fields, numeric stats, items (replaces entirely), contingency_prompts (replaces entirely).

**Not overridable**: booleans can only be set to `true` — you cannot override a template's `true` to `false`. Attributes and CombatMods merge (add/replace keys; cannot remove a key). If you need to override a boolean to `false` or strip a stat, create a separate template.

---

## Core Principles

### 1. Disposition Must Earn Its Place

`disposition` is injected directly next to the NPC's name in every narrator prompt. It is the narrator's first and fastest signal about how to play this character. A generic value is wasted space.

| Generic (reject) | Specific (accept) |
|---|---|
| `"friendly"` | `"warmly suspicious — helpful until given a reason not to be"` |
| `"hostile"` | `"hostile to pirates"` |
| `"neutral"` | `"bored and contemptuous"` |
| `"gruff but helpful"` | `"impatient with fools but fair to those who know their trade"` |
| `"mysterious"` | `"friendly but mysterious"` |

Ask: if the narrator read only this word, would they know how to handle a conversation? If not, revise.

### 2. Description Must Be Narrator-Ready

`description` is the narrator's only persistent, always-available source of information about who this character is. It must allow the narrator to:

- Describe the NPC's presence in a room
- Write their dialogue rhythm and vocabulary
- Portray them consistently across turns

**Required elements** (both modes):
- Physical anchor: a detail that makes this character visually or behaviorally distinct
- Personality: what they want, guard, or fear; how they treat people
- Speech marker: accent, vocabulary, rhythm, or a sample phrase

**Two description modes — choose before writing:**

**Summarized** (default): One dense paragraph weaving all three elements together. The narrator gets a fast, complete read. Ideal for ambient, supporting, or lightly featured NPCs. Aim for 2–4 sentences.

**Detailed**: Three paragraphs, each dedicated to one element. Use for significant NPCs the narrator will portray across many turns — romance interests, primary antagonists, long-term allies.
1. Physical traits — age, build, hair, eyes, clothing, scars, notable features
2. Psychological profile — core desire, fear, behavioral tendencies, how they change under pressure
3. Speech patterns — accent, vocabulary, sentence rhythm; include 2–3 sample dialogue excerpts

Do not mix modes. A Summarized description should weave all elements into one paragraph. A Detailed description should give each paragraph enough depth to stand alone as narrator guidance.

| Weak (reject) | Summarized (accept) |
|---|---|
| `"A merchant who sells things."` | `"A merchant who deals in black-market goods and smells of fish; he grumbles constantly but has what you need."` |
| `"A mysterious bartender."` | `"A bartender known for her enchanting stories and elusive nature; she speaks with a Haitian accent and a hint of hidden knowledge."` |
| `"A loyal first mate."` | `"The player's loyal first mate with a keen sense of duty; speaks with nautical slang and trusts superstition over science."` |
| `"An old pirate."` | `"A grizzled old pirate with a wooden leg and a talent for turning every conversation into a story about himself."` |

If a description could apply to any NPC of that type, it is not doing its job.

### 3. No Story Bleed

`description` and `contingency_prompts` are injected on every turn the NPC is co-located with the player. They must be true across the **entire** session — not just at a plot moment.

**Story bleed (reject):**
- References to specific plot events that may or may not have occurred
- Future-tense descriptions of what the NPC will do
- Conditional states that belong in a `contingency_prompt` with a `when` guard
- References to scenario-specific named NPCs that this NPC "knows" or "fears"

**Allowed:**
- Fixed personality traits, speech patterns, attitudes
- Backstory that is always true (a scar, a job, a long-standing grudge with a type of person, not a specific named character)
- Behavioral tendencies that hold regardless of story state

**The test:** Could this description and all `contingency_prompts` (without `when` guards) be read aloud at the very start of the game without being false?

### 4. Contingency Prompts Are Behavior, Not Plot, and are for use INLINE only

Only use contingency prompts for inline scenario prompting, and never in a template. Each contingency prompt must describe **how the NPC acts** — not what happens next in the story.

| Plot (reject) | Behavior (accept) |
|---|---|
| `"Calypso will give the map when the player proves worthy."` | `"Calypso is slow to trust and will deflect personal questions with riddles or a change of subject."` |
| `"The shipwright needs the repair ledger."` | `"The shipwright is direct about requirements and will not begin work until they are met."` |
| `"Gibbs knows about the treasure."` | `"Gibbs speaks with nautical slang and occasionally refers to superstitions as a substitute for facts."` |

Use **conditional prompts** (`when` guards) for state-dependent behavior changes. Without a guard, a prompt is injected on every turn.

```json
"contingency_prompts": [
  "Calypso speaks with a subtle Haitian accent and mystical undertones.",
  {
    "prompt": "Calypso eyes you with particular interest, as if she knows more than she lets on.",
    "when": { "vars": { "ship_repair_ledger_acquired": "false" } }
  }
]
```

### 5. `IsImportant` Is Never Set in Sidecar Templates; Use Sparingly in Inline NPCs

Setting `"important": true` causes the NPC to appear in the narrator prompt **even when they are not in the same location as the player**. This leads to NPC bleed. Never set it for ambient or supporting characters. Never set it in a sidecar template.

---

## Field-by-Field Standards

### `name`
The display name used in prompts and system output. Use title case. Keys are lowercase snake_case.

### `type`
Engine metadata only — **not narrator-visible**. Use for your own organization. Do not put narrator-relevant information here that you want in the prompt; put it in `description`.

### `disposition`
Injected as `"(disposition)"` in narrator prompt. 1 short phrase. Must be specific enough to guide performance. See Principle 1.

### `description`
The primary character profile injected into every narrator prompt. Choose one mode before writing:

**Summarized** — One paragraph, 2–4 sentences, weaving physical, personality, and speech into a unified quick-read. Use for most NPCs.

**Detailed** — Three paragraphs separated by `\n\n` in the JSON string:
1. **Physical**: Age, size, build, sex, hair, eyes, clothing, scars or notable features
2. **Psychology**: Core desire or fear, how they relate to others, how they behave under stress
3. **Speech**: Accent, vocabulary level, verbal habits; include 2–3 sample dialogue excerpts in quotes

In Detailed mode each paragraph should be 3–5 sentences — enough for the narrator to sustain a voice across many turns.

Write in present tense, third-person. Plain evocative language — the narrator supplies the stylistic filter.

### `location`
A valid location key from the scenario. Required for the NPC to appear in location-filtered prompts.

Always OMIT in sidecars.

### `items`
Items the NPC possesses. These appear in the narrator prompt. Keep to items the narrator should mention or that the player may acquire. Do not list items for internal tracking only.

### `contingency_prompts`
Behavioral/voice guidance injected only while the player is co-located. See Principle 4. 2–4 prompts ideal. Use `when` guards for state-dependent additions.

### `following`
`"pc"` or an NPC key. The engine auto-syncs location — the LLM does not need to move this NPC manually. Do not set for NPCs that should stay put.

### Actor properties (sidecar only)
`ac`, `hp`, `max_hp`, `attributes`, `combat_modifiers`, `drop_items_on_defeat` — optional even in sidecar files. Only include if the NPC is intended to be combatable or if stats affect gameplay. When present, they appear in the narrator prompt as `[AC: X, HP: Y/Z]`.

---

## Procedure: Create an Inline NPC

1. **Establish purpose**: What role does this NPC play in the scenario? Information source, obstacle, ally, vendor?
2. **Write `disposition`**: 1 phrase, specific. Read it aloud — would an actor know how to play this character?
3. **Write `description`**: 2–3 sentences. Physical anchor, personality, speech marker. Run the story-bleed test.
4. **Set `location`**: A valid location key.
5. **Set `items`**: Only items the narrator should know about or the player may acquire.
6. **Write `contingency_prompts`**: 2–4 items. Behavior first. Add `when`-guarded entries for state changes.
7. **Story-bleed check**: Scan every text field for plot events, future-tense, named scenario NPCs/places. Remove or generalize.
8. **Vagueness check**: Every adjective in `description` and every prompt — could it apply to any NPC? If yes, make it specific or delete it.

## Procedure: Create a Sidecar NPC Template

Same as inline, plus:

1. **File placement**: `data/npcs/{template_id}.json` — lowercase snake_case filename, no spaces.
2. **Set `TemplateID`** to the filename (without `.json`) — the storage layer sets this automatically, but good practice to include it in the file.
3. **Design for reuse**: `description` and `contingency_prompts` must be scenario-agnostic. No references to specific scenario locations, events, or other named NPCs.
4. **Actor properties**: Include only if this NPC is intended to be combatable. Match HP/AC to comparable monsters in `data/monsters/`. Use D&D 5e attributes (8 = weak, 10 = average, 14 = trained, 16 = exceptional).
5. **Scenario reference**: In the scenario file, set `template_id` and supply only instance-specific overrides (location, disposition variant).

## Procedure: Review an Existing NPC

1. Read every text field.
2. Check `disposition`: specific or generic? Would an actor know how to perform this?
3. Check `description`: narrator-ready (physical anchor, personality, speech)? Story-bleed-free? Identify the mode — Summarized (1 paragraph, 2–4 sentences) or Detailed (3 paragraphs, 3–5 sentences each) — and flag if it falls short of its own mode's depth requirements.
4. Check each `contingency_prompt`: behavior or plot? Specific enough to bind narrator output? Does it need a `when` guard?
5. Check `type`: is narrator-relevant information accidentally buried here instead of in `description`?
6. Check `IsImportant`: is this flag actually justified?
7. Check `items`: are all listed items ones the narrator should know about?
8. Report findings. Ask before rewriting unless the user asked for a full revision.

---

## Quality Checklist

Before finalizing any NPC:

- [ ] `disposition` is specific — not a generic adjective
- [ ] `description` has a physical or behavioral anchor + personality marker
- [ ] `description` mode is chosen: Summarized (1 paragraph, 2–4 sentences) or Detailed (3 paragraphs, 3–5 sentences each)
- [ ] `description` has no plot references or future-tense statements
- [ ] `type` is not being used to communicate narrator-relevant information
- [ ] `contingency_prompts` describe behavior, not plot outcomes
- [ ] No `contingency_prompt` without a `when` guard references a plot state that may not yet be true
- [ ] `contingency_prompts` count is 2–4
- [ ] `IsImportant` is only set when genuinely necessary
- [ ] `location` is a valid key present in the scenario's locations map
- [ ] All JSON is valid: double quotes only, no trailing commas, commas between all array/object members

### Sidecar-specific:
- [ ] File is in `data/npcs/`, named `{template_id}.json`
- [ ] No scenario-specific references in any text field
- [ ] Actor properties present only if NPC is combatable

---

## JSON Validity Rules

Common errors that break parsing:

```json
// ❌ Trailing comma after last item in object or array
"items": ["sword", "shield",]

// ❌ Single quotes instead of double quotes
'name': 'Guard Captain'

// ❌ Missing comma between object keys
{
  "name": "Calypso"
  "type": "bartender"
}

// ❌ Unescaped double quote inside a string
"description": "She says "hello" to everyone."

// ✅ Escaped correctly
"description": "She says \"hello\" to everyone."

// ❌ Missing comma between array elements
"contingency_prompts": [
  "She speaks softly."
  "She never answers directly."
]

// ✅ Correct
"contingency_prompts": [
  "She speaks softly.",
  "She never answers directly."
]
```

---

## Examples: Weak vs Strong

### Inline NPC — Weak

```json
"shipwright": {
  "name": "Shipwright",
  "type": "shipwright",
  "disposition": "gruff",
  "description": "A shipwright who repairs ships for money.",
  "location": "sleepy_mermaid",
  "contingency_prompts": [
    "The shipwright needs the repair ledger and 500 gold to fix the ship.",
    "He will give the player a receipt when he's paid."
  ]
}
```

**Problems:**
- `disposition`: `"gruff"` tells the narrator nothing specific — does he yell? ignore? speak in clipped sentences?
- `description`: no physical anchor, no speech marker, pure function-label
- `contingency_prompts`: both describe plot mechanics, not behavior — the narrator reads these as stage directions, not voice guidance. Plot mechanics belong in `contingency_rules`, not `contingency_prompts`.

### Inline NPC — Strong

```json
"shipwright": {
  "name": "Shipwright",
  "type": "shipwright",
  "disposition": "impatient with fools but fair to those who know their trade",
  "description": "A burly man with calloused hands and sawdust perpetually in his beard. He sketches repair designs on scraps of paper and quotes prices before you finish asking the question.",
  "location": "sleepy_mermaid",
  "contingency_prompts": [
    "The shipwright is direct and business-like; he has no patience for charm or negotiation.",
    "He speaks in practical terms — cost, time, materials — and ignores everything else.",
    {
      "prompt": "The shipwright taps his fingers impatiently, clearly waiting for something specific before he will engage.",
      "when": { "vars": { "ship_repair_ledger_acquired": "false" } }
    }
  ]
}
```

**Why it works:**
- `disposition`: specific behavioral contract — the narrator knows exactly where the line is
- `description`: physical (calloused hands, sawdust), behavioral (sketches, quotes prices), implies speech rhythm
- `contingency_prompts`: first two are always-on behavioral anchors; third is state-guarded and avoids plot leakage when the ledger has already been acquired

---

### Sidecar NPC — Weak

```json
{
  "name": "City Guard",
  "type": "guard",
  "disposition": "hostile",
  "description": "A guard who patrols the city and stops criminals.",
  "contingency_prompts": [
    "The guard will arrest the player if they cause trouble.",
    "Guards are dangerous and well-armed."
  ]
}
```

**Problems:**
- `disposition`: `"hostile"` without context — hostile to whom, in what way?
- `description`: functional label, no physical presence, no speech style
- `contingency_prompts`: first is a plot directive (what will happen), not a behavioral anchor; second is a threat notice with no voice guidance

### Sidecar NPC — Strong

```json
{
  "name": "Guard Captain",
  "type": "guard",
  "disposition": "authoritative",
  "description": "The captain of the city guard — disciplined, dangerous, and incorruptible. She has served for twenty years and lost nothing of her edge.",
  "items": ["longsword", "badge of office"],
  "ac": 16,
  "hp": 45,
  "max_hp": 45,
  "attributes": {
    "strength": 16,
    "dexterity": 12,
    "constitution": 14,
    "intelligence": 10,
    "wisdom": 13,
    "charisma": 12
  },
  "combat_modifiers": {
    "longsword": 5
  },
  "drop_items_on_defeat": true,
  "contingency_prompts": [
    "The Guard Captain is vigilant and enforces the law without hesitation.",
    "If threatened or attacked, the Guard Captain calls for reinforcements and does not back down."
  ]
}
```

**Why it works:**
- `disposition`: a single word with weight — implies authority, not malice
- `description`: two sentences; first gives physical/professional anchor, second gives history that explains the edge
- `contingency_prompts`: first establishes always-on posture; second gives escalation behavior without dictating plot outcomes
- Actor stats are present because this NPC is intended to be combatable

---

## Reference

- NPC struct: `pkg/actor/npc.go`
- Template merge logic: `actor.NewNPCFromTemplate()` in `pkg/actor/npc.go`
- Standalone templates: `data/npcs/`
- Scenario NPC reference: `docs/guide-for-scenarios.md` → "NPCs" section
- Contingency prompt conditions: `vars`, `location`, `turn_counter`, `min_turns`, `scene_turn_counter`, `min_scene_turns`
