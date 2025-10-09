# Scenario Writing Guide

This guide explains how to create scenarios for the story engine. A scenario is a JSON file that defines an interactive story with locations, NPCs, items, and game mechanics.

## Basic Structure

Every scenario must include these top-level fields:

```json
{
  "name": "Scenario Title",
  "story": "Brief description of the scenario premise",
  "rating": "PG-13",
  "opening_scene": "scene_id",
  "opening_prompt": "Narrator text shown directly to the player",
  "opening_location": "location_id", 
  "opening_inventory": ["item1", "item2"],
  "locations": { /* optional location definitions */ },
  "npcs": { /* optional NPC definitions */ },
  "scenes": { /* optional scene system */ },
  "contingency_prompts": [ /* narrative guidance */ ],
  "contingency_rules": [ /* game logic rules */ ],
  "game_end_prompt": "Final evaluation text"
}
```

## Writing Voice and Perspective

- **Most content**: Write in third person referring to "the player"
  - Example: "The player's ship badly needs repairs"
- **Opening prompt**: Write as the narrator speaking directly to the player  
  - Example: "You are the captain of The Black Pearl..."

## Locations

Locations define the game world geography:

```json
"locations": {
  "Tortuga": {
    "name": "Tortuga",
    "description": "A bustling pirate port filled with taverns, traders, and trouble.",
    "exits": {
      "east": "Black Pearl",
      "south": "Sleepy Mermaid",
      "sewer grate": "Sewer System"
    },
    "blocked_exits": {
      "north": "Lots of British soldiers in northern docks."
    },
    "items": ["barrels of goods", "abandoned knapsack"]
  }
}
```

- **exits**: Available movement options (direction: destination)
- **blocked_exits**: Inaccessible exits with explanation why
- **items**: Objects available for pickup in this location

## NPCs (Non-Player Characters)

NPCs bring the world to life and drive story interactions:

```json
"npcs": {
  "Calypso": {
    "name": "Calypso",
    "type": "bartender",
    "disposition": "friendly but mysterious", 
    "description": "A bartender known for her enchanting stories and elusive nature. Speaks with a Haitian accent.",
    "important": true,
    "location": "Sleepy Mermaid",
    "items": ["flagon of ale", "deck of cards"]
  }
}
```

- **type**: Role/profession of the NPC
- **disposition**: Personality and attitude toward the player
- **description**: Physical appearance and notable characteristics
- **important**: Whether this NPC is crucial to the story progression
- **location**: Current location of the NPC
- **items**: Objects this NPC possesses

## Contingency System

The contingency system provides two types of guidance:

### Contingency Prompts (Narrative Guidance)

These provide storytelling direction to the AI narrator:

```json
"contingency_prompts": [
  "When the player boards the black pearl, if the repair ledger is not in inventory, mention the repair ledger when describing the ship's condition.",
  "The shipwright can BEGIN work for a small deposit. To FINISH the repairs, he requires payment of 500 gold doubloons.",
  "Calypso says: \"Ah, dee Black Pearl needs much work, mon.\"",
  "Use some humor in responses."
]
```

- Describe **when** and **how** to present information
- Include specific NPC dialogue by using quotes: `NPCName says: "exact dialogue"`
- Guide tone, mood, and storytelling style
- Provide contextual hints and suggestions

### Contingency Rules (Game Logic)

These define hard mechanical rules that change game state:

```json
"contingency_rules": [
  "When the shipwright starts repairs, the scene changes to 'british_docks'.",
  "Reading the ship repair ledger adds it to inventory.",
  "Showing the ship repair ledger to the shipwright removes it from the player's inventory.", 
  "If the Black Pearl leaves Tortuga in disrepair, the ship sinks and the game ends."
]
```

- Use precise conditional language: "When X happens, Y occurs"
- Define state changes: inventory modifications, scene transitions, location moves
- Specify win/lose conditions and game endings
- Control NPC behavior and availability

### Language Patterns for Rules

- **Conditional**: "When [condition], [effect]"
- **State changes**: "adds it to inventory", "removes it from inventory", "scene changes to"
- **Movement**: "NPC moves to [location]"
- **Game flow**: "game ends", "scene transitions to"
- **Availability**: "becomes accessible", "is blocked"

### Variables (Vars)

Variables track important story state and enable deterministic scene transitions. Use them **sparingly** - primarily to scaffold critical story progression points.

**Define variables at the scene level:**
```json
"scenes": {
  "shipwright": {
    "vars": {
      "ship_repair_ledger_acquired": "false",
      "shipwright_hired": "false"
    }
  }
}
```

**Use clear, descriptive names:**
- `ship_repair_ledger_acquired` - Better than `got_ledger` or `ledger_flag`
- `shipwright_hired` - Better than `hired` or `npc_status`
- Use snake_case format
- All values are strings: `"true"`, `"false"`, `"ready"`, `"incomplete"`

**Provide narrative guidance for setting variables:**
```json
"contingency_prompts": [
  "When the player reviews the ship repair ledger, set ship_repair_ledger_acquired to true.",
  "When the shipwright agrees to begin repairs, set shipwright_hired to true."
]
```

### Conditionals (Deterministic Scene Changes)

Conditionals enforce reliable scene transitions based on variable state. They override any scene changes suggested by the AI.

**Define conditionals at the scene level:**
```json
"scenes": {
  "shipwright": {
    "conditionals": [
      {
        "name": "transition_to_british_docks",
        "when": {
          "shipwright_hired": "true"
        },
        "then": {
          "scene": "british_docks"
        }
      }
    ]
  }
}
```

**Multiple conditions (all must be true):**
```json
"conditionals": [
  {
    "name": "proceed_to_finale",
    "when": {
      "gold_acquired": "true",
      "shipwright_paid_in_full": "true"
    },
    "then": {
      "scene": "calypsos_map"
    }
  }
]
```

**Conditionals can also end the game:**
```json
"conditionals": [
  {
    "name": "game_over_captured",
    "when": {
      "caught_by_guards": "true",
      "disguise_acquired": "false"
    },
    "then": {
      "game_ended": true
    }
  }
]
```

### Best Practice: Combine Narrative and Deterministic Approaches

For reliable scene progression, use **both** contingency prompts and conditionals:

**1. Guide the AI with contingency prompts:**
```json
"contingency_prompts": [
  "When the shipwright agrees to begin repairs and accepts payment, set shipwright_hired to true."
]
```

**2. Provide a narrative scene change rule:**
```json
"contingency_rules": [
  "When the shipwright starts repairs, the scene changes to 'british_docks'."
]
```

**3. Enforce it with a conditional:**
```json
"conditionals": [
  {
    "name": "shipwright_scene_transition",
    "when": {"shipwright_hired": "true"},
    "then": {"scene": "british_docks"}
  }
]
```

This layered approach ensures:
- The AI understands when and why to set variables
- The AI attempts scene transitions naturally through contingency rules
- The conditional guarantees the transition happens regardless of AI compliance
- Story progression remains reliable and predictable

## Scene System (Optional)

The scene system helps keep complex stories on track by defining story phases. About 3 scenes works well for most scenarios.

```json
"scenes": {
  "shipwright": {
    "story": "The player's ship badly needs repairs...",
    "locations": { /* scene-specific location overrides */ },
    "npcs": { /* scene-specific NPC overrides */ },
    "contingency_prompts": [ /* scene-specific narrative guidance */ ],
    "contingency_rules": [ /* scene-specific game logic */ ]
  }
}
```

### Story and Scenes
The scene-scoped story prompt *augments* the scenario-scoped prompt. That is, both are used in the system prompt. 

### Scene Overrides

- **Scene-level definitions *override* scenario-level definitions**
- New Locations can be defined at the scene level. If so, they are scoped to the scene.
- Locations can have different descriptions, exits, or items per scene.
- New NPCS can similarly be defined at the scene level, and scoped to the scene.
- NPCs can move locations, change disposition, or gain/lose items.
- This allows the world to evolve as the story progresses.

### Where to Place Locations and NPCs: Scenario vs Scene Level

**Place at the scenario level when:**
- The location or NPC appears in most or all scenes
- The location or NPC is central to the story
- The location or NPC doesn't change much throughout the story

**Place at the scene level when:**
- The location or NPC should NOT appear in all scenes
- The location or NPC is only relevant to specific story phases
- The location or NPC needs significant changes between scenes (new exits, completely different items, etc.)

Example: In a castle scenario, place the "Grand Foyer" at scenario level since players access it throughout the story, but place "Dracula's Sanctum" only in the final confrontation scene.

### Contingency System and Scenes
Any scene-scoped contingency rules and prompts *augment* the scenario. That is, both are used in the final system prompt. 

## Best Practices

### Story Design
- Create clear objectives for the player
- Use about 3 scenes for manageable complexity
- Build logical progression between scenes
- Include multiple paths to success when possible

### Writing Style  
- Keep descriptions vivid but concise
- Give NPCs distinct personalities and speech patterns
- Use contingency prompts to reinforce atmosphere and tone
- Use contingency prompts to provide extra context for important NPCs
- Balance helpful guidance with player agency

### Technical Considerations
- Use scene overrides to show world changes over time
- Scene changes are critical, and models can make mistakes; add fallback prompts to progress the scene in case the main trigger is missed
- Place important items and NPCs strategically
- If an item is important to the story, give it a contingency prompt
- Design clear fail states and victory conditions

### Common Patterns
- **Gated progression**: Use contingency rules to require certain actions before scene changes
- **Inventory puzzles**: Items needed to progress or unlock content  
- **Social interactions**: NPCs who provide information, items, or services
- **Environmental storytelling**: Let location descriptions convey backstory and mood

## Example Flow

1. Player starts in opening location with opening inventory
2. Contingency prompts guide initial presentation
3. Player explores, interacts with NPCs, collects items
4. Contingency rules trigger scene changes based on player actions
5. Scene overrides modify world state for new story phase
6. Process repeats until endgame conditions are met
7. Game end prompt evaluates player performance

Remember: The goal is creating an engaging, entertaining narrative where player choices matter and the world responds dynamically to their actions.
