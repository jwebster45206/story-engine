# Scenario Writing Guide

This guide explains how to create scenarios for the story engine. A scenario is a JSON file that defines an interactive story with locations, NPCs, items, and game mechanics.

## Basic Structure

Every scenario must include these top-level fields:

```json
{
  "name": "Scenario Title",
  "story": "Brief description of the scenario premise",
  "rating": "PG-13",
  "narrator_id": "vincent_price",
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

## Narrator (Optional)

Scenarios can specify a narrator to define the storytelling voice and style. Narrators are reusable personalities stored in separate JSON files in the `data/narrators/` directory.

```json
{
  "name": "Haunted Manor",
  "narrator_id": "vincent_price",
  ...
}
```

**Available narrators:**
- `classic` - Traditional, straightforward adventure narrator
- `vincent_price` - Dramatic, theatrical Gothic horror style
- `noir` - Cynical, hard-boiled detective style
- `comedic` - Lighthearted, humorous narrator

**If no narrator is specified**, the story uses a standard omniscient narrator voice.

**Creating custom narrators:** See `data/narrators/README.md` for details on creating your own narrator personalities.

## Writing Voice and Perspective

- **Most content**: Write in third person referring to "the player"
  - Example: "The player's ship badly needs repairs"
- **Opening prompt**: Write as the narrator speaking directly to the player  
  - Example: "You are the captain of The Black Pearl..."
- **Narrator personality**: If using a narrator_id, the narrator's voice will automatically influence how the story is told

## Locations

Locations define the game world geography:

```json
"locations": {
  "tortuga": {
    "name": "Tortuga",
    "description": "A bustling pirate port filled with taverns, traders, and trouble.",
    "exits": {
      "east": "black_pearl",
      "south": "sleepy_mermaid",
      "sewer grate": "sewer_system"
    },
    "blocked_exits": {
      "north": "Lots of British soldiers in northern docks."
    },
    "items": ["barrels of goods", "abandoned knapsack"]
  }
}
```

### Location Naming Conventions

Use **lowercase snake_case** for location keys (e.g., `"black_pearl"`, `"captains_cabin"`). These are internal IDs used in exits, NPC locations, and game state. The `"name"` field is for display text and can use any formatting (e.g., `"Black Pearl"`, `"Captain's Cabin"`).

```json
"locations": {
  "black_pearl": {
    "name": "Black Pearl",
    "exits": {
      "cabin door": "captains_cabin",
      "west": "tortuga"
    }
  },
  "captains_cabin": {
    "name": "Captain's Cabin",
    "exits": {
      "cabin door": "black_pearl"
    }
  }
}
```

### Location Fields

- **exits**: Available movement options (direction: destination)
- **blocked_exits**: Inaccessible exits with explanation why
- **items**: Objects available for pickup in this location
- **important**: Whether the location should always appear in gamestate prompts (generally should be omitted/false)

## NPCs (Non-Player Characters)

NPCs bring the world to life and drive story interactions:

```json
"npcs": {
  "calypso": {
    "name": "Calypso",
    "type": "bartender",
    "disposition": "friendly but mysterious", 
    "description": "A bartender known for her enchanting stories and elusive nature. Speaks with a Haitian accent.",
    "location": "sleepy_mermaid",
    "items": ["flagon of ale", "deck of cards"]
  }
}
```

### NPC Naming Conventions

Use **lowercase snake_case** for NPC keys (e.g., `"calypso"`, `"charming_danny"`). These are internal IDs used in game state and item operations. The `"name"` field is for display text and can use any formatting (e.g., `"Calypso"`, `"Charming Danny"`).

```json
"npcs": {
  "charming_danny": {
    "name": "Charming Danny",
    "location": "tortuga_market",
    "items": ["bottle of rum"]
  },
  "captain_morgan": {
    "name": "Captain Morgan",
    "location": "black_pearl"
  }
}
```

The game engine will accept both the NPC key (ID) and the display name in item operations, so the LLM can use either:
- `give "rum" to "charming_danny"` (using ID)
- `give "rum" to "Charming Danny"` (using display name)

Both will work correctly.

### NPC Fields

- **type**: Role/profession of the NPC
- **disposition**: Personality and attitude toward the player
- **description**: Physical appearance and notable characteristics
- **important**: Whether this NPC should always appear in gamestate prompts (generally should not be true)
- **location**: Current location of the NPC (use location ID)
- **items**: Objects this NPC possesses

## Contingency System

The contingency system provides two types of guidance that serve **different purposes and different audiences**:

### Contingency Prompts (Narrative Hints for the AI Narrator)

**Purpose**: Guide how the story is *told* and *presented* to the player  
**Audience**: The LLM generating narrative responses  
**Effect**: Influences storytelling style, dialogue, and narrative flow

**Important Limitations**: Contingency prompts are **hints and suggestions only**. The LLM does not prioritize them and may choose to ignore them entirely. They provide contextual guidance but cannot enforce specific behaviors. For deterministic game mechanics (inventory changes, scene transitions, variable updates), use **contingency rules** instead.

**For Deterministic Narrative Moments**: If you need the narrator to describe a specific event at a precise moment (like "lightning strikes the tower" on turn 4), use **story events** instead of contingency prompts. Story events are injected directly into the conversation stream and treated as mandatory narrative directives by the AI.

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

## Conditional Logic (When Clauses)

Both **contingency prompts** and **story events** can use conditional logic to control when they activate. The `when` clause supports the following conditions:

### Supported Conditional Types

**1. Variable Conditions** - Trigger when specific variables have certain values:
```json
"when": {
  "vars": {
    "opened_grimoire": "true",
    "found_key": "true"
  }
}
```
All variables in the `vars` map must match for the condition to be true.

**2. Location Conditions** - Trigger when player is at a specific location:
```json
"when": {
  "location": "Castle Gates"
}
```

**3. Scene Turn Counter (Exact Match)** - Trigger on a specific turn within the current scene:
```json
"when": {
  "scene_turn_counter": 4
}
```
Triggers **only** on turn 4 of the current scene.

**4. Global Turn Counter (Exact Match)** - Trigger on a specific turn of the entire game:
```json
"when": {
  "turn_counter": 10
}
```
Triggers **only** on turn 10 of the game.

**5. Minimum Scene Turns (Threshold)** - Trigger after a certain number of turns in the current scene:
```json
"when": {
  "min_scene_turns": 5
}
```
Triggers on turn 5 **and all subsequent turns** in the current scene.

**6. Minimum Global Turns (Threshold)** - Trigger after a certain number of turns in the game:
```json
"when": {
  "min_turns": 20
}
```
Triggers on turn 20 **and all subsequent turns** of the game.

**7. Combined Conditions** - All conditions must be true:
```json
"when": {
  "location": "Castle Gates",
  "min_scene_turns": 3,
  "vars": {
    "wolves_appeared": "true"
  }
}
```

### Turn Counter Reference

- `turn_counter` / `min_turns`: Counts turns across the **entire game** (never resets)
- `scene_turn_counter` / `min_scene_turns`: Counts turns in the **current scene only** (resets when scene changes)

### Exact vs Minimum

- `turn_counter` / `scene_turn_counter`: Triggers **only on that specific turn** (exact match)
- `min_turns` / `min_scene_turns`: Triggers **from that turn onward** (threshold)

### Conditional Contingency Prompts

Contingency prompts can include conditionals to control **when** they are shown to the AI narrator:

**Format:**
```json
"contingency_prompts": [
  "Always-active prompt string",
  {
    "prompt": "Conditional prompt text",
    "when": { /* conditions - see above */ }
  }
]
```

**Important Note**: Since contingency prompts are hints rather than commands, conditional prompts provide **contextual suggestions** but do not guarantee specific LLM behavior. Use story events for guaranteed narrative moments.

**Using `min_scene_turns` as a Safeguard:**

One powerful use of `min_scene_turns` is preventing players from getting stuck in a scene:

```json
{
  "prompt": "If the player seems stuck, the NPC should suggest: 'Perhaps you should examine the painting more closely.'",
  "when": {"min_scene_turns": 8}
}
```

## Story Events (Deterministic Narrative Moments)

**Story events** provide guaranteed, priority narrative moments that appear at precisely the right time in your story. Unlike contingency prompts (which are hints), story events are **injected directly into the conversation stream** and treated as mandatory narrative directives by the AI narrator.

### Story Events vs Contingency Prompts

| Feature | Story Events | Contingency Prompts |
|---------|--------------|---------------------|
| **Reliability** | Guaranteed to appear | May be ignored by LLM |
| **Timing** | Precise (fires once when triggered) | Continuous suggestion while active |
| **Delivery** | Injected into chat as priority message | Added to system prompt context |
| **Use Case** | Critical plot moments, dramatic beats | Atmospheric guidance, storytelling hints |
| **Persistence** | One-time (clears after firing) | Continuous (remains while condition true) |

**When to Use Story Events:**
- Critical plot moments that MUST happen: "The villain appears!"
- Dramatic beats with precise timing: "Lightning strikes on turn 4"
- One-time reveals or transformations: "The statue comes to life"
- Surprise interruptions: "An explosion rocks the building"

**When to Use Contingency Prompts:**
- General storytelling tone and style
- Ongoing atmospheric suggestions
- NPC personality hints
- Location description guidance
- Flexible narrative nudges

### Defining Story Events

Story events are defined **within individual scenes** using the `story_events` map:

```json
"scenes": {
  "castle_arrival": {
    "story": "The player approaches Castle Ravenloft...",
    "story_events": {
      "dracula_materializes": {
        "prompt": "Count Dracula materializes from the shadows, his eyes burning with ancient hunger. His presence fills the room with oppressive, supernatural dread.",
        "when": {
          "vars": {
            "opened_grimoire": "true"
          }
        }
      },
      "lightning_strike": {
        "prompt": "A massive LIGHTNING bolt strikes the castle tower! Thunder shakes the very stones beneath your feet! The air crackles with electricity.",
        "when": {
          "scene_turn_counter": 4
        }
      }
    }
  }
}
```

### Story Event Naming Conventions

Use **lowercase snake_case** for story event keys (e.g., `"dracula_materializes"`, `"lightning_strike"`). These are internal IDs used for debugging and logging.

**Each story event has:**
- **Key**: The event ID in snake_case (used as the map key)
- `prompt`: The exact narrative text that will be injected into the story
- `when`: Conditional logic determining when the event triggers (see **Conditional Logic** section above for all supported conditions)

### How Story Events Work

**1. Evaluation (Turn N):**
When a player submits an action, the engine evaluates all story events in the current scene against the game state. Any events whose conditions are met are added to a queue.

**2. Injection (Turn N+1):**
On the **next turn**, queued story events are injected into the conversation history as a special assistant/agent message:

```
User: I examine the grimoire carefully.
Assistant: [Story event injected here]
STORY EVENT: Count Dracula materializes from the shadows, his eyes burning with ancient hunger.
User: [Current player action]
```

The LLM sees this as a mandatory narrative directive and incorporates it into the response.

**3. Clearing:**
After injection, the story event is cleared from the queue. It will **never fire again** (one-time use).

### Writing Effective Story Events

**Be Descriptive and Complete:**
Story events should contain the full narrative beat you want to occur. The LLM will incorporate this into the response naturally.

**❌ Too Vague:**
```json
"prompt": "Dracula appears"
```

**✅ Descriptive and Atmospheric:**
```json
"prompt": "Count Dracula materializes from the shadows, his eyes burning with ancient hunger. His presence fills the room with oppressive, supernatural dread. He speaks: 'Welcome to my domain, mortal.'"
```

**Use Present Tense and Active Voice:**
Events describe what's happening right now in the story.

**❌ Past or Future Tense:**
```json
"prompt": "Lightning will strike the tower" // Future
"prompt": "Lightning struck the tower" // Past
```

**✅ Present Tense:**
```json
"prompt": "A massive LIGHTNING bolt strikes the castle tower! Thunder shakes the very stones beneath your feet!"
```

**Include Sensory Details:**
Good story events engage multiple senses and create atmosphere.

```json
"prompt": "A swarm of bats erupts from the ceiling, their screeching filling the air! The wind from thousands of wings buffets your face as they circle overhead."
```

**Use Emphasis for Impact:**
CAPITALIZE key words or use punctuation to convey drama and urgency.

```json
"prompt": "The ground EXPLODES beneath your feet! Chunks of stone rain down as a massive creature bursts from below!"
```

### Multiple Events in One Turn

If multiple story events trigger on the same turn, they are all injected together, separated by double newlines:

```
STORY EVENT: Count Dracula materializes from the shadows...

STORY EVENT: A swarm of bats erupts from the ceiling...
```

The LLM will incorporate both events into its response.

### Complete Example

Here's a scene with story events, contingency prompts, rules, and conditionals working together:

```json
"scenes": {
  "castle_confrontation": {
    "story": "The player has entered Castle Ravenloft to confront Count Dracula.",
    "vars": {
      "opened_grimoire": "false",
      "dracula_appeared": "false"
    },
    "story_events": [
      {
        "name": "dracula_materializes",
        "prompt": "Count Dracula materializes from the shadows, his eyes burning with ancient hunger. \"So, another fool seeks to challenge me.\"",
        "when": {"vars": {"opened_grimoire": "true"}}
      },
      {
        "name": "lightning_strike",
        "prompt": "A MASSIVE lightning bolt strikes the tower! Thunder shakes the castle!",
        "when": {"scene_turn_counter": 4}
      }
    ],
    "contingency_prompts": [
      "Describe the castle as ancient, foreboding, and filled with supernatural dread.",
      "Dracula speaks with formal, aristocratic language from centuries past."
    ],
    "contingency_rules": [
      "When the player opens or reads the ancient grimoire, set opened_grimoire to true.",
      "If the player defeats or escapes Dracula, the scene changes to 'epilogue'."
    ],
    "conditionals": [
      {
        "name": "victory_transition",
        "when": {"dracula_defeated": "true"},
        "then": {"scene": "epilogue"}
      }
    ]
  }
}
```

**In this example:**
- **Story events** provide dramatic one-time moments with precise timing
- **Contingency prompts** guide ongoing atmosphere and tone  
- **Contingency rules** handle state changes (variables, scene transitions)
- **Conditionals** enforce deterministic scene transitions

### Best Practices for Story Events

✅ **Use for critical plot moments** that absolutely must occur
✅ **Write complete, vivid descriptions** with sensory details
✅ **Use present tense and active voice** for immediacy
✅ **Combine with variables** to track that events have occurred
✅ **Time events carefully** using appropriate conditionals
✅ **Keep events scene-specific** - they only evaluate in their defined scene

❌ **Don't use for general atmosphere** (use contingency prompts instead)
❌ **Don't use for state changes** (use contingency rules instead)
❌ **Don't make events fire repeatedly** (they auto-clear after injection)
❌ **Don't write vague events** - be specific and descriptive

### Story Events vs Conditionals vs Contingency Rules

It's important to understand when to use each system:

**Story Events**: "What dramatic narrative moment should occur?"
```json
"story_events": [
  {
    "name": "betrayal",
    "prompt": "Captain Morgan suddenly draws his pistol and aims it at you! 'I'm sorry, but the treasure means more to me than friendship.'",
    "when": {"vars": {"treasure_revealed": "true"}}
  }
]
```

**Contingency Rules**: "What mechanical state changes should happen?"
```json
"contingency_rules": [
  "When Captain Morgan betrays the player, set morgan_hostile to true and morgan moves to Black Pearl.",
  "If the player reveals the treasure location to Morgan, set treasure_revealed to true."
]
```

**Conditionals**: "What scene transition should be enforced?"
```json
"conditionals": [
  {
    "name": "betrayal_scene",
    "when": {"morgan_hostile": "true"},
    "then": {"scene": "betrayal_confrontation"}
  }
]
```

All three work together to create reliable, dramatic storytelling.

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
- Avoid the use of colon or ":" character; it has a special use in the console view

### Technical Considerations
- Use scene overrides to show world changes over time
- Scene changes are critical, and models can make mistakes; add fallback prompts to progress the scene in case the main trigger is missed
- Place important items and NPCs strategically
- If an item is important to the story, give it a contingency prompt
- Design clear fail states and victory conditions
- Short item names are easier for the LLM to follow: example: "pieces of eight" rather than "captain jimmy's last pieces of eight"

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
