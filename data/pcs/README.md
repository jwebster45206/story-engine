# Player Character (PC) Creation Guide

This guide explains how to create Player Characters (PCs) for the story engine. PCs define the character that the player controls during gameplay, including their stats, abilities, background, and narrative identity.

## Overview

PCs are stored as JSON files in the `data/pcs/` directory. Each PC represents a complete character definition that can be used across multiple scenarios. The filename (without `.json`) becomes the PC's ID.

## Basic Structure

Every PC file must include these fields:

```json
{
  "id": "fighter_01",
  "name": "Aric of Durnholde",
  "class": "Fighter",
  "level": 1,
  "race": "Human",
  "pronouns": "he/him",
  "description": "A one-sentence character summary for narrative use",
  "background": "A longer backstory that provides depth and context",
  "stats": {
    "strength": 16,
    "dexterity": 13,
    "constitution": 15,
    "intelligence": 10,
    "wisdom": 12,
    "charisma": 8
  },
  "hp": 12,
  "max_hp": 12,
  "ac": 16,
  "combat_modifiers": {
    "strength": 3,
    "proficiency": 2
  },
  "attributes": {
    "athletics": 5,
    "perception": 3
  },
  "inventory": [
    "longsword",
    "shield"
  ]
}
```

## Field Reference

### Core Identity

- **id** (string, overridden by filename): Internal identifier. The filename always takes precedence. Should be lowercase and snake_case. 
- **name** (string, required): The character's full name as it appears in the story.
- **class** (string, optional): Character class or profession (e.g., "Fighter", "Rogue", "Merchant"). 
- **level** (integer, optional): Character level (typically 1-20 for D&D-style characters).
- **race** (string, optional): Character race or species (e.g., "Human", "Elf", "Dwarf").
- **pronouns** (string, recommended): Character pronouns (e.g., "he/him", "she/her", "they/them"). Used by the narrator.
- **description** (string, recommended): A concise 1-2 sentence summary of the character. Used in narrative prompts.
- **background** (string, optional): Extended backstory and personality details. Provides rich context for storytelling.

### Combat Stats (D&D 5e Compatible)

- **stats** (object, required): The six core ability scores:
  - **strength** (integer): Physical power and melee combat
  - **dexterity** (integer): Agility, reflexes, and ranged combat
  - **constitution** (integer): Endurance and hit points
  - **intelligence** (integer): Reasoning and knowledge
  - **wisdom** (integer): Awareness and insight
  - **charisma** (integer): Force of personality and social skills
  
  Standard range: 3-18 for mortals, 1-30 for extraordinary beings. Average is 10-11.

- **hp** (integer, *required*): Current hit points.
- **max_hp** (integer, *required*): Maximum hit points.
- **ac** (integer, *required*): Armor Class (higher is harder to hit). Typical range: 10-20.

### Advanced Attributes

- **combat_modifiers** (object, optional): Modifiers applied to attack rolls.
  - Common modifiers: `"strength"`, `"dexterity"`, `"proficiency"`, `"magic_weapon"`, etc.
  - Example: `{"strength": 3, "proficiency": 2}` adds +5 to melee attacks
  
- **attributes** (object, optional): Skills, proficiencies, and custom abilities.
  - D&D 5e skills: `"athletics"`, `"acrobatics"`, `"stealth"`, `"perception"`, `"investigation"`, etc.
  - Custom skills: `"navigation"`, `"sea_lore"`, `"streetwise"`, etc.
  - Values typically range from -5 to +15

- **inventory** (array of strings, optional): Starting items the character possesses.
  - Items are simple strings, not quantities
  - Example: `["longsword", "shield", "rope", "torch"]`

## Character Creation Guidelines

### 1. Naming Conventions

**Filename:** Use `lowercase_snake_case` for filenames:
- ✅ `pirate_captain.json`
- ✅ `elven_ranger.json`
- ✅ `classic.json`
- ❌ `Pirate Captain.json`
- ❌ `pirate-captain.json`

**Character Name:** Use proper capitalization and spacing:
- ✅ `"Captain Jack Sparrow"`
- ✅ `"Aric of Durnholde"`
- ✅ `"The Wanderer"`

### 2. Writing Descriptions

**Description Field** (1-2 sentences):
- Focus on immediate, recognizable traits
- Mention personality, appearance, or reputation
- Keep it concise for narrative injection

Good examples:
```json
"description": "A notorious pirate captain known for his cunning and unpredictable nature."
```
```json
"description": "A disciplined soldier who distrusts magic and seeks to protect the weak."
```

**Background Field** (2-5 paragraphs):
- Provide detailed backstory
- Include personality quirks and motivations
- Explain relationships and goals
- Use vivid, story-friendly language

### 3. Stat Guidelines

**For D&D 5e Characters:**
- Use standard array: 15, 14, 13, 12, 10, 8
- Or point buy (27 points, max 15 before racial bonuses)
- Apply racial bonuses if applicable
- Average human: all stats at 10

**For Generic/Custom Characters:**
- Use 10 as baseline (average)
- 8-9: Below average
- 12-13: Above average
- 14-15: Talented
- 16-18: Exceptional
- 19+: Legendary

**HP Calculation (D&D 5e):**
- Base HP = Hit die + Constitution modifier
- Hit dice: d6 (Wizard), d8 (Rogue), d10 (Fighter), d12 (Barbarian)
- Example: Fighter with 15 CON = 10 (d10) + 2 (CON mod) = 12 HP

**AC Calculation (D&D 5e):**
- Unarmored: 10 + DEX modifier
- Light armor: 11-12 + DEX modifier
- Medium armor: 13-15 + DEX modifier (max +2)
- Heavy armor: 14-18 (no DEX)
- Shield: +2

### 4. Modifiers and Attributes

**Combat Modifiers:**
- Ability modifier: (Ability Score - 10) / 2, rounded down
- Proficiency bonus: +2 (level 1-4), +3 (level 5-8), +4 (level 9-12), etc.
- Total = Ability modifier + Proficiency (if proficient)

Example for level 5 Fighter with STR 16:
```json
"combat_modifiers": {
  "strength": 3,      // (16-10)/2 = +3
  "proficiency": 3    // Level 5 = +3
}
// Total attack bonus: +6
```

**Skill Attributes:**
- Proficient skills: Ability modifier + Proficiency bonus
- Non-proficient skills: Just ability modifier (or omit)

Example for level 5 Rogue with DEX 18:
```json
"attributes": {
  "stealth": 7,        // +4 DEX + 3 proficiency
  "perception": 4,     // +1 WIS + 3 proficiency
  "acrobatics": 4      // +4 DEX (not proficient)
}
```

## Character Archetypes

### Generic/Classic PC

For scenarios that don't require specific character details:

```json
{
  "id": "classic",
  "name": "Adventurer",
  "class": "Adventurer",
  "level": 1,
  "race": "Human",
  "pronouns": "they/them",
  "description": "A capable adventurer seeking fortune and glory.",
  "stats": {
    "strength": 12,
    "dexterity": 12,
    "constitution": 12,
    "intelligence": 12,
    "wisdom": 12,
    "charisma": 12
  },
  "hp": 10,
  "max_hp": 10,
  "ac": 12,
  "inventory": ["sword", "backpack"]
}
```

### D&D 5e Fighter

```json
{
  "id": "fighter_01",
  "name": "Aric of Durnholde",
  "class": "Fighter",
  "level": 3,
  "race": "Human",
  "pronouns": "he/him",
  "description": "A disciplined soldier who distrusts magic and fights to protect the weak.",
  "background": "Veteran of the northern border wars, Aric learned discipline and honor in the shield wall. He seeks to earn a captain's commission while staying true to his principles.",
  "stats": {
    "strength": 16,
    "dexterity": 13,
    "constitution": 15,
    "intelligence": 10,
    "wisdom": 12,
    "charisma": 8
  },
  "hp": 26,
  "max_hp": 26,
  "ac": 18,
  "combat_modifiers": {
    "strength": 3,
    "proficiency": 2
  },
  "attributes": {
    "athletics": 5,
    "intimidation": 1,
    "perception": 3
  },
  "inventory": ["longsword", "shield", "plate armor"]
}
```

### Rogue/Thief

```json
{
  "id": "rogue_01",
  "name": "Mira Shadowstep",
  "class": "Rogue",
  "level": 5,
  "race": "Halfling",
  "pronouns": "she/her",
  "description": "A nimble thief with a silver tongue and quick fingers.",
  "background": "Mira grew up on the streets, learning to survive by wit and stealth. She has a code: steal from the rich, help the desperate, and never leave a friend behind.",
  "stats": {
    "strength": 8,
    "dexterity": 18,
    "constitution": 12,
    "intelligence": 14,
    "wisdom": 13,
    "charisma": 16
  },
  "hp": 28,
  "max_hp": 28,
  "ac": 15,
  "combat_modifiers": {
    "dexterity": 4,
    "proficiency": 3
  },
  "attributes": {
    "stealth": 10,
    "sleight_of_hand": 10,
    "acrobatics": 7,
    "perception": 6,
    "persuasion": 6,
    "deception": 9
  },
  "inventory": ["dagger", "thieves' tools", "dark cloak"]
}
```

### Non-D&D Character (Modern Setting)

```json
{
  "id": "detective_01",
  "name": "Alexandra Kane",
  "class": "Private Investigator",
  "level": 5,
  "race": "Human",
  "pronouns": "she/her",
  "description": "A sharp-eyed detective with a reputation for solving impossible cases.",
  "background": "Former police detective turned private investigator after uncovering corruption in the department. She combines street smarts with academic knowledge to unravel mysteries others can't solve.",
  "stats": {
    "strength": 10,
    "dexterity": 14,
    "constitution": 12,
    "intelligence": 16,
    "wisdom": 15,
    "charisma": 13
  },
  "hp": 30,
  "max_hp": 30,
  "ac": 12,
  "attributes": {
    "investigation": 9,
    "perception": 8,
    "insight": 7,
    "persuasion": 5,
    "intimidation": 4,
    "streetwise": 6
  },
  "inventory": ["pistol", "notepad", "flashlight", "lockpicks"]
}
```

## Best Practices

### 1. Make Characters Memorable
- Give them distinctive traits, quirks, or mannerisms
- Include specific motivations and goals
- Provide concrete backstory details
- Use vivid, evocative language

### 2. Balance Stats for Gameplay
- Not every character needs to be powerful
- Weaknesses create interesting roleplay opportunities
- Match stats to the character concept
- Consider the scenario's difficulty

### 3. Consider Pronouns
- Alwys include pronouns for proper narration
- Respect the character's identity
- "they/them" works well for generic characters
- Narrator will use these consistently

### 5. Inventory Management
- Start with essential gear only
- Items should match the character and scenario
- Keep the list small and prioritized (1-3 items)

## Technical Notes

### JSON Format
- All PC files must be valid JSON
- Use UTF-8 encoding
- Include all required fields
- Optional fields can be omitted entirely

### Filename = ID
The filename (without `.json`) becomes the PC's ID, overriding any `"id"` field in the JSON. This ensures consistency:
- `pirate_captain.json` → ID is `"pirate_captain"`
- `elven_ranger.json` → ID is `"elven_ranger"`

### Runtime Behavior
When a PC is loaded:
1. The spec is parsed from JSON
2. A `d20.Actor` is built with all stats and modifiers
3. The Actor is used for any combat or skill checks
4. The Spec is used for narrative and API responses

### Connecting PCs to Scenarios
Scenarios can specify a default PC using the `"default_pc"` field:
```json
{
  "name": "Pirate Adventure",
  "default_pc": "pirate_captain",
  ...
}
```

If no PC is specified, the system falls back to `"classic"`.

See `data/scenarios/README.md` for more details on scenario configuration.

## Examples in This Directory

- **classic.json** - Generic adventurer for any scenario
- **pirate_captain.json** - Captain Jack Sparrow for pirate scenarios
- **alexandra_kane.json** - Modern detective for mystery scenarios

Study these examples when creating your own characters.

## Validation

Before using a PC, ensure:
- ✅ Valid JSON syntax
- ✅ All required fields present
- ✅ Stats are reasonable numbers (typically 3-20)
- ✅ HP and AC are positive integers
- ✅ Pronouns are specified
- ✅ Description is narrative-friendly
- ✅ Filename uses snake_case

## Testing Your PC

After creating a PC:
1. Test via API: `GET /v1/pcs/{id}`
2. Create a game state with your PC
3. Ask narrative questions like "Who am I?" to verify the narrator uses PC details
4. Check that stats and inventory work correctly in gameplay

## Further Reading

- **Scenario README**: `data/scenarios/README.md` - How to connect PCs to scenarios
- **Narrator README**: `data/narrators/README.md` - How narrators interact with PCs
- **D&D 5e SRD**: For D&D-compatible stat blocks
- **Main README**: Project overview and API documentation
