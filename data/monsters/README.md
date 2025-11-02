# Monster Templates

This directory contains monster template definitions for the Story Engine. Monster templates define the base attributes, combat stats, and behavior for creatures that can appear in scenarios.

## Monster System v1 (Combat-Lite)

The current monster system focuses on **lifecycle management** rather than full tactical combat:
- Monsters can spawn and despawn based on conditions
- Basic HP/AC tracking for narrative combat
- Inventory/loot drops on defeat
- Location-based filtering (monsters only appear in prompts when player is in same location)

## Template Structure

Monster templates are JSON files. Here's a minimal example:

```json
{
  "template_id": "rat",
  "name": "Rat",
  "description": "A small, mangy rat with beady eyes.",
  "ac": 6,
  "hp": 1,
  "max_hp": 1
}
```

And a more complete example with attributes and combat modifiers:

```json
{
  "template_id": "orc_warrior",
  "name": "Orc Warrior",
  "description": "A hulking orc clad in crude iron armor, brandishing a notched battleaxe. Battle scars crisscross its green skin.",
  "ac": 13,
  "hp": 15,
  "max_hp": 15,
  "attributes": {
    "strength": 16,
    "dexterity": 12,
    "constitution": 16,
    "intelligence": 7,
    "wisdom": 11,
    "charisma": 10
  },
  "combat_modifiers": {
    "greataxe": 5,
    "javelin": 5
  },
  "items": ["rusty greataxe"],
  "drop_items_on_defeat": true
}
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `template_id` | string | Yes | Unique identifier for this template (typically matches filename) |
| `name` | string | Yes | Display name shown to players |
| `description` | string | Yes | Narrative description of the creature's appearance and demeanor |
| `ac` | integer | Yes | Armor Class - difficulty to hit (typical range: 6-20) |
| `hp` | integer | Yes | Starting hit points |
| `max_hp` | integer | Yes | Maximum hit points (usually same as hp) |
| `attributes` | object | No | D20-style attributes (strength, dexterity, constitution, etc.) |
| `combat_modifiers` | object | No | Attack bonuses by attack type (e.g., "bite": 3, "claw": 1) |
| `items` | array | No | Items the monster is carrying |
| `drop_items_on_defeat` | boolean | No | Whether items should drop when defeated (default: false) |

## Naming Conventions

- **Template ID**: Use lowercase with underscores (e.g., `giant_rat`, `skeletal_warrior`, `orc_warrior`)
- **Filename**: Match template_id with `.json` extension (e.g., `giant_rat.json`, `orc_warrior.json`)
- **Display Name**: Use proper capitalization (e.g., "Giant Rat", "Skeletal Warrior", "Orc Warrior")

## When to Use Monsters vs NPCs

**Use Monsters for:**
- Generic enemy types that may appear multiple times (rats, orcs, zombies, guards)
- Combat encounters with simple mechanics (hit points, armor class)
- Creatures without complex dialogue or personality
- Enemies that are primarily obstacles or threats

**Use NPCs instead for:**
- **Unique boss encounters** with complex behavior and dialogue (Count Dracula, Dragon Lord)
- Named characters with significant story roles (Captain Blackbeard, Necromancer Zul'jin)
- Enemies that need to negotiate, flee, or make complex decisions
- Antagonists with personality, motivation, and character development

**Example:** A cave might have 5 "goblin" monsters (using the template), but the Goblin King should be an NPC with unique dialogue, items, and story importance.

## Combat Stats Guidelines

### Armor Class (AC)
- **6-8**: Very easy to hit (small vermin, weak creatures)
- **10-12**: Easy to hit (common monsters, unarmored humanoids)
- **13-15**: Moderate difficulty (armored foes, quick creatures)
- **16-18**: Hard to hit (heavily armored, very agile)
- **19-20**: Very hard to hit (legendary creatures, elite guards)

### Hit Points (HP)
- **1-3**: One-hit minions (rats, insects, fragile undead)
- **4-10**: Weak monsters (goblins, zombies, common beasts)
- **11-25**: Standard monsters (orcs, wolves, bandits)
- **26-50**: Tough monsters (ogres, vampires, elite soldiers)
- **51+**: Boss monsters (dragons, giants, demon lords)

## Usage in Scenarios

Monsters can be placed in scenarios in two ways:

### 1. Pre-placed in Locations

Define monsters that exist when the game starts or scene loads:

```json
{
  "locations": {
    "cellar": {
      "name": "Dark Cellar",
      "description": "A dank cellar filled with shadows.",
      "monsters": {
        "rat_1": {
          "template_id": "rat"
        },
        "rat_2": {
          "template_id": "rat"
        },
        "alpha_rat": {
          "template_id": "giant_rat"
        }
      }
    }
  }
}
```

### 2. Conditional Spawn/Despawn

Spawn or remove monsters based on game events:

```json
{
  "conditionals": {
    "rat_appears": {
      "when": {
        "vars": {"disturbed_nest": "true"}
      },
      "then": {
        "monster_events": [
          {
            "action": "spawn",
            "instance_id": "surprise_rat",
            "template": "giant_rat",
            "location": "ship_deck"
          }
        ],
        "prompt": "STORY EVENT: A massive rat scurries out from the cargo hold!"
      }
    },
    "rat_flees": {
      "when": {
        "vars": {"rat_wounded": "true"}
      },
      "then": {
        "monster_events": [
          {
            "action": "despawn",
            "instance_id": "surprise_rat"
          }
        ],
        "prompt": "STORY EVENT: The wounded rat flees into the darkness."
      }
    }
  }
}
```

### Template Overrides

You can override template values when spawning:

```json
{
  "action": "spawn",
  "instance_id": "buffed_rat",
  "template": "giant_rat",
  "location": "boss_room",
  "name": "Rat King",
  "description": "An enormous rat wearing a crude crown, surrounded by smaller rats.",
  "hp": 20,
  "max_hp": 20,
  "ac": 14,
  "items": ["tiny_crown", "cheese_wheel"]
}
```

## How Monsters Appear in Gameplay

1. **Location Filtering**: Monsters only appear in the LLM prompt when the player is in the same location
2. **Prompt Format**: Monsters are shown as:
   ```
   MONSTERS:
   - Giant Rat (AC: 12, HP: 7/7): A massive rat, the size of a large dog...
   ```
3. **Narrator Guidance**: The LLM is instructed to:
   - Mention monsters naturally in the narrative
   - Use the description for flavor text
   - NOT allow players to invent new monsters
   - Handle combat outcomes based on HP/AC

## Examples

See existing templates for reference:
- `rat.json` - Minimal template (only required fields)
- `giant_rat.json` - Simple creature with basic stats
- `orc_warrior.json` - Complete template with attributes, combat modifiers, and loot (example above)

For complex, story-important enemies like bosses, use NPCs instead of monster templates.

## Testing

Validate your monster templates:

```bash
go run cmd/validate/main.go data/scenarios/your_scenario.json
```

Test monster behavior in integration tests:

```bash
go test -tags=integration ./integration -case your_test -v
```
