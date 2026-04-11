# Standalone NPC Templates

This directory contains reusable NPC definitions that can be referenced from
scenarios via `"template_id"`. Think of them as the NPC equivalent of monster
templates in `data/monsters/` — a shared character library that any scenario
can pull from.

---

## Why standalone NPCs?

Inline NPCs (defined entirely inside a scenario's `"npcs"` map) are still fully
supported and unchanged. Use standalone templates when you want to:

- **Reuse a character** across multiple scenarios without copy-pasting definitions.
- **Give an NPC actor properties** (HP, AC, combat stats) for a richer gameplay
  experience — e.g., a villain who can be fought.
- **Keep scenarios lean** — scenarios only need to supply the instance-specific
  bits (starting location, a disposition override) while the character definition
  lives here.

---

## File format

Each NPC template is a JSON file named `{template_id}.json`.

### Minimal template (narrative-only)

```json
{
  "name": "The Old Innkeeper",
  "type": "innkeeper",
  "disposition": "friendly",
  "description": "A weathered but warm-hearted old man who has seen it all."
}
```

### Full template (with actor properties)

```json
{
  "name": "Guard Captain",
  "type": "guard",
  "disposition": "authoritative",
  "description": "The captain of the city guard — disciplined, dangerous, and incorruptible.",
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
    "The Guard Captain is vigilant and will enforce the law without hesitation.",
    "If attacked, the Guard Captain calls for reinforcements."
  ]
}
```

---

## Referencing a template in a scenario

In your scenario's `"npcs"` map, set `"template_id"` to the filename (without
`.json`). Inline fields are treated as **overrides** and replace the
corresponding template fields at game-start. Only specify what you want to
change; everything else comes from the template.

```json
"npcs": {
  "captain": {
    "template_id": "guard_captain",
    "location": "city_gate",
    "disposition": "suspicious"
  }
}
```

At game creation the engine will:
1. Load `data/npcs/guard_captain.json`
2. Merge the inline definition on top (here: set location to `"city_gate"` and
   override disposition to `"suspicious"`)
3. Use the merged NPC for the rest of the session

If the template file is missing the engine logs a warning and falls back to the
inline definition.

---

## Actor properties

Actor properties (`ac`, `hp`, `max_hp`, `attributes`, `combat_modifiers`,
`drop_items_on_defeat`) are **optional**. A template without them is perfectly
valid — it's just a reusable narrative character. Actor stats appear in the
prompt output alongside those of monsters when present.

See `docs/guide-for-scenarios.md` for the complete NPC reference.
