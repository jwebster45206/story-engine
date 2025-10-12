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

The contingency system provides two types of guidance that serve **different purposes and different audiences**:

### Contingency Prompts (Narrative Hints for the AI Narrator)

**Purpose**: Guide how the story is *told* and *presented* to the player  
**Audience**: The LLM generating narrative responses  
**Effect**: Influences storytelling style, dialogue, and narrative flow

**Important Limitations**: Contingency prompts are **hints and suggestions only**. The LLM does not prioritize them and may choose to ignore them entirely. They provide contextual guidance but cannot enforce specific behaviors. For deterministic game mechanics (inventory changes, scene transitions, variable updates), use **contingency rules** instead.

TODO: We don't have a system for injecting priority story events yet. It's coming soon. 

These are hints and suggestions for the AI narrator about how to present the story:

```json
"contingency_prompts": [
  "When the player boards the black pearl, if the repair ledger is not in inventory, mention the repair ledger when describing the ship's condition.",
  "The shipwright can BEGIN work for a small deposit. To FINISH the repairs, he requires payment of 500 gold doubloons.",
  "Calypso says: \"Ah, dee Black Pearl needs much work, mon.\"",
  "Use some humor in responses."
]
```

**Use contingency_prompts for:**
- Describing **when** and **how** to present information to the player
- Providing specific NPC dialogue using quotes: `NPCName says: "exact dialogue"`
- Guiding tone, mood, and storytelling style
- Offering contextual hints and narrative suggestions
- Reminding the AI about story details or character behaviors
- Suggesting what to emphasize or mention in certain situations

**Think of these as**: "Hey narrator, here's how to tell this part of the story..."

**Remember**: These are suggestions, not commands. The LLM may incorporate them, paraphrase them, or ignore them based on its interpretation of the narrative context.

#### Conditional Contingency Prompts

Contingency prompts can include **conditionals** to control **when** they are shown to the AI narrator. This allows you to provide contextual narrative guidance that only appears when specific conditions are met.

**Important Note**: Since contingency prompts are hints rather than commands, conditional prompts provide **contextual suggestions** but do not guarantee specific LLM behavior. The LLM may still choose to ignore or reinterpret conditional prompts based on its understanding of the narrative. Conditional prompts are most effective when:
- Combined with deterministic contingency rules for critical mechanics
- Used for atmospheric enhancements rather than plot-critical events
- Designed as helpful nudges rather than strict requirements

**Format:**
```json
"contingency_prompts": [
  "Always-active prompt string",
  {
    "prompt": "Conditional prompt text",
    "when": {
      /* conditions */
    }
  }
]
```

**Supported Conditional Types:**

**1. Variable Conditions** - Show prompt when specific variables have certain values:
```json
{
  "prompt": "Count Dracula materializes from the shadows, his eyes burning with ancient hunger. His presence fills the room with an oppressive, supernatural dread.",
  "when": {
    "vars": {
      "opened_grimoire": "true"
    }
  }
}
```
All variables in the `vars` map must match for the prompt to be shown.

**2. Location Conditions** - Show prompt only when player is at a specific location:
```json
{
  "prompt": "A pack of dire wolves with glowing yellow eyes emerges from the forest, blocking the path forward.",
  "when": {
    "location": "Castle Gates"
  }
}
```

**3. Scene Turn Counter (Exact Match)** - Show prompt on a specific turn within the current scene:
```json
{
  "prompt": "A massive LIGHTNING bolt strikes the castle tower! Thunder shakes the very stones beneath your feet!",
  "when": {
    "scene_turn_counter": 4
  }
}
```
This shows the prompt **only** on turn 4 of the current scene.

**4. Global Turn Counter (Exact Match)** - Show prompt on a specific turn of the entire game:
```json
{
  "prompt": "You sense something significant is about to happen.",
  "when": {
    "turn_counter": 10
  }
}
```
This shows the prompt **only** on turn 10 of the game.

**5. Minimum Scene Turns (Threshold)** - Show prompt after a certain number of turns in the current scene:
```json
{
  "prompt": "You've been here for a while. Perhaps it's time to move on or try something different.",
  "when": {
    "min_scene_turns": 5
  }
}
```
This shows the prompt on turn 5 **and all subsequent turns** in the current scene.

**6. Minimum Global Turns (Threshold)** - Show prompt after a certain number of turns in the game:
```json
{
  "prompt": "Your journey has been long. Victory or defeat must be approaching.",
  "when": {
    "min_turns": 20
  }
}
```
This shows the prompt on turn 20 **and all subsequent turns** of the game.

**7. Combined Conditions** - All conditions must be true for the prompt to show:
```json
{
  "prompt": "The wolves grow more aggressive the longer you linger here.",
  "when": {
    "location": "Castle Gates",
    "min_scene_turns": 3,
    "vars": {
      "wolves_appeared": "true"
    }
  }
}
```

**Turn Counter vs Scene Turn Counter:**
- `turn_counter` / `min_turns`: Counts turns across the **entire game** (never resets)
- `scene_turn_counter` / `min_scene_turns`: Counts turns in the **current scene only** (resets when scene changes)

**Exact vs Minimum:**
- `turn_counter` / `scene_turn_counter`: Shows **only on that specific turn** (exact match)
- `min_turns` / `min_scene_turns`: Shows **from that turn onward** (threshold)

**Using `min_scene_turns` as a Safeguard:**

One powerful use of `min_scene_turns` is preventing players from getting stuck in a scene. If your scene requires a specific action that the player might miss, you can provide an escape hatch:

```json
{
  "prompt": "If the player seems stuck or has been in this scene for many turns, the NPC should directly suggest: 'Perhaps you should examine the painting more closely' or provide another clear hint about the hidden lever.",
  "when": {
    "min_scene_turns": 8
  }
}
```

Or automatically transition the scene:

```json
{
  "prompt": "After spending considerable time here, you notice a previously hidden passage. Describe this clearly and suggest the player can move forward through it.",
  "when": {
    "min_scene_turns": 10
  }
}
```

This ensures players won't be permanently stuck if they miss a critical clue or action. The story can gracefully guide them forward after a reasonable number of attempts.

**Complete Example:**

```json
"scenes": {
  "castle_arrival": {
    "contingency_prompts": [
      "Describe the castle as ancient and foreboding.",
      {
        "prompt": "Count Dracula materializes from the shadows with supernatural presence.",
        "when": {
          "vars": {
            "opened_grimoire": "true"
          }
        }
      },
      {
        "prompt": "Dire wolves block your path with glowing yellow eyes.",
        "when": {
          "location": "Castle Gates",
          "min_scene_turns": 3
        }
      },
      {
        "prompt": "LIGHTNING strikes the castle tower! Thunder shakes the stones!",
        "when": {
          "scene_turn_counter": 4
        }
      },
      {
        "prompt": "If the player seems stuck, have the local guide suggest examining the old grimoire on the altar.",
        "when": {
          "min_scene_turns": 8
        }
      }
    ]
  }
}
```

**Best Practices for Conditional Prompts:**
- Use variable conditions for story branches triggered by player actions
- Use location conditions for location-specific atmospheric details
- Use exact turn counters (`scene_turn_counter`, `turn_counter`) for dramatic one-time events
- Use minimum turn thresholds (`min_scene_turns`, `min_turns`) for:
  - Progressive hints that intensify over time
  - Safety nets to prevent players getting stuck
  - Atmospheric buildup that grows with time spent
- Combine conditions to create highly specific contextual guidance
- Keep conditional prompts focused on narrative guidance, not state changes (use contingency_rules for state changes)

### Contingency Rules (State Change Instructions for the Game Engine)

**Purpose**: Define what mechanically *happens* in the game state  
**Audience**: The state reducer/game engine processing player actions  
**Effect**: Actually modifies game state (inventory, scenes, variables, game over)

These are imperative instructions that trigger concrete state changes:

```json
"contingency_rules": [
  "When the shipwright agrees to start repairs, set the variable \"shipwright_hired\" to \"true\".",
  "Reading the ship repair ledger adds it to inventory and sets the variable \"ship_repair_ledger_acquired\" to \"true\".",
  "Showing the ship repair ledger to the shipwright removes it from the player's inventory.", 
  "If the Black Pearl leaves Tortuga in disrepair, the ship sinks and the game ends."
]
```

**Use contingency_rules for:**
- Precise conditional logic: "When X happens, Y occurs"
- State changes: adding/removing inventory items, setting variables
- Scene transitions: "the scene changes to 'scene_id'"
- Game endings: "the game ends"
- NPC location changes: "NPC moves to location_id"
- Any mechanical game state modification

**Think of these as**: "Hey game engine, here's what actually changes in the game state..."

### Key Differences

| Aspect | Contingency Prompts | Contingency Rules |
|--------|-------------------|------------------|
| **Target** | AI Narrator | Game Engine/Reducer |
| **Purpose** | Narrative guidance | State modification |
| **Effect** | Influences storytelling | Changes game state |
| **Language** | Suggestive ("mention", "should", "can") | Imperative ("adds to", "removes from", "sets", "changes to") |
| **Examples** | Tone, dialogue, descriptions, hints | Inventory, variables, scene changes, game over |

### Common Mistake to Avoid

❌ **Wrong**: Putting state changes in contingency_prompts  
```json
"contingency_prompts": [
  "When the player reads the ledger, add it to inventory"  // NO! This is a state change
]
```

✅ **Correct**: State changes belong in contingency_rules  
```json
"contingency_rules": [
  "Reading the ship repair ledger adds it to inventory."  // YES! Actual state change
]
```

❌ **Wrong**: Putting narrative guidance in contingency_rules  
```json
"contingency_rules": [
  "The shipwright should sound gruff but helpful"  // NO! This is narrative guidance
]
```

✅ **Correct**: Narrative guidance belongs in contingency_prompts  
```json
"contingency_prompts": [
  "The shipwright speaks in a gruff but helpful manner."  // YES! Storytelling hint
]
```

### Language Patterns for Rules

- **Conditional**: "When [condition], [effect]"
- **State changes**: "adds it to inventory", "removes it from inventory", "scene changes to"
- **Movement**: "NPC moves to [location]"
- **Game flow**: "game ends", "scene transitions to"
- **Availability**: "becomes accessible", "is blocked"

### Writing Rules for Reliable LLM Behavior

LLMs can be inconsistent at interpreting abstract conditions. Make your contingency rules **explicit and action-focused** to improve reliability:

**❌ Too Abstract:**
```json
"When the Black Pearl sails from Tortuga, set the variable \"pearl_departed_tortuga\" to \"true\"."
```
*Problem: "Sails from Tortuga" doesn't clearly map to player input patterns*

**✅ Explicit and Action-Focused:**
```json
"When the player commands the crew to sail, set sail, weigh anchor, depart, or leave Tortuga (or uses the 'open sea' exit from the Black Pearl), set the variable \"pearl_departed_tortuga\" to \"true\"."
```
*Better: Lists concrete player actions and exit names that should trigger the condition*

**Key principles:**
- **List specific player verbs**: "talk to", "show [item] to", "give [item] to", "pick up", "read"
- **Include exit names**: Reference actual exit keys from your location definitions
- **Avoid indirection**: If the player is already at a location, saying "go to that location" is unclear
- **Multiple triggers**: Use "or" to list alternative actions that should have the same effect
- **Be concrete about consequences**: Use exact item names, location names, variable names

**More examples:**

**❌ Vague:**
```json
"If the player helps the NPC, give them a reward."
```

**✅ Specific:**
```json
"When the player gives the 'lost ring' to the merchant OR completes the merchant's delivery quest, add 'bag of gold coins' to inventory and set the variable \"merchant_helped\" to \"true\"."
```

**❌ Abstract:**
```json
"When the puzzle is solved, open the door."
```

**✅ Concrete:**
```json
"When the player places the 'ruby key' in the door's lock OR speaks the password 'mellon' at the Ancient Door, remove 'Ancient Door' from the blocked_exits for the Hall and set the variable \"vault_accessible\" to \"true\"."
```

### Variables (Vars)

Variables track important story state and enable deterministic scene transitions. Use them **only** to scaffold critical story progression points via conditionals. 

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
  "The shipwright is agreeable to most forms of downpayment and ready to start work quickly."
]
```

**2. Provide a narrative scene change rule and a separate variable rule:**
```json
"contingency_rules": [
  "When the shipwright starts repairs, the scene changes to 'british_docks'.",
  "When the shipwright agrees to begin repairs and accepts payment, set shipwright_hired to true."
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
- The story stays on its intended guiderails.
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
