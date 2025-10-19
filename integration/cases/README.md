# Integration Test Cases

This directory contains integration test cases for the story engine. Each test case is a JSON file that defines a complete test scenario with initial state, steps, and expectations.

## Test Case Structure

### Basic Test Format

```json
{
  "name": "Test Case Name",
  "description": "Brief description of what this test validates",
  "scenario": "scenario-file.json",
  "seed_game_state": {
    "scene_name": "scene_id",
    "user_location": "location_id",
    "turn_counter": 2,
    "scene_turn_counter": 2,
    "user_inventory": ["item1", "item2"],
    "vars": {
      "variable_name": "value"
    },
    "chat_history": [
      {
        "role": "user",
        "content": "Previous user action"
      },
      {
        "role": "assistant",
        "content": "Previous AI response"
      }
    ]
  },
  "steps": [
    {
      "name": "Step description",
      "user_prompt": "What the player says or does",
      "expect": {
        "scene_name": "expected_scene",
        "user_location": "expected_location_id",
        "turn_counter": 3,
        "vars": {
          "variable_name": "expected_value"
        },
        "response_contains": ["text1", "text2"]
      }
    }
  ]
}
```

### Rollup Test Format

Rollup tests run multiple test cases in sequence. They can even include other rollups for nested test suites:

```json
{
  "name": "Test Suite Name",
  "description": "Runs multiple related tests",
  "cases": [
    "test_case_1.json",
    "test_case_2.json",
    "another_rollup.json"
  ]
}
```

## Seed Game State

The `seed_game_state` sets up the initial conditions before running test steps.

### Required Fields
- `scene_name`: Current scene ID from the scenario
- `user_location`: Player's current location
- `turn_counter`: Global turn count (use 2 if seeding 2 chat pairs)
- `scene_turn_counter`: Turn count within current scene
- `chat_history`: Array of previous conversation turns

### Optional Fields
- `user_inventory`: Items the player has (default: empty array)
- `vars`: Game variables and their values (default: empty object)

### Turn Counter Pattern

For consistent turn counting, follow this pattern:
- Seed with **2 chat history pairs** (user + assistant, user + assistant)
- Set `turn_counter: 2` and `scene_turn_counter: 2`
- First test step becomes turn 3

**Why?** The turn counter increments *after* the AI responds. Starting at turn 2 with 2 pairs means the seed state is consistent with what the engine would produce after 2 actual turns.

### Chat History Best Practices

- **Use 2 conversation pairs** to establish context without overwhelming the test
- **Write narrative responses** that feel like actual game output
- **Show player progression** through the world (movement, discovery, interaction)
- **Avoid dry/mechanical language** - make it feel like real gameplay
- **Stay in-character** - player prompts should sound natural, not like test commands

**Good Example:**
```json
"chat_history": [
  {
    "role": "user",
    "content": "I walk through the gates and enter the castle."
  },
  {
    "role": "assistant",
    "content": "The heavy iron gates creak open as you push through them, and you step into the Grand Foyer. The oppressive atmosphere immediately envelops you - dusty portraits line the walls, their pale subjects seeming to watch your every move."
  }
]
```

**Bad Example:**
```json
"chat_history": [
  {
    "role": "user",
    "content": "go to location"
  },
  {
    "role": "assistant",
    "content": "You are now at the location. You see things."
  }
]
```

### What NOT to Seed

In most cases, you **don't need to seed** these fields:
- `world_locations`: Scenario loading handles this
- `npcs`: Scenario loading handles this
- `contingency_prompts`: Scenario loading handles this

Only seed these if your test specifically requires **overriding** scenario defaults.

## Test Steps

Each test step sends a user prompt and validates the response and resulting game state.

### Step Fields

- `name`: Brief description of what this step tests
- `user_prompt`: The message sent to the engine (player's action or dialogue)
- `expect`: Object defining what to validate after this step

### Special Step: Reset Game State

You can reset the game state back to the seed data mid-test:

```json
{
  "name": "Reset to beginning",
  "user_prompt": "RESET_GAMESTATE",
  "expect": {
    "scene_name": "arrival",
    "turn_counter": 2
  }
}
```

This is useful for testing multiple branches from the same starting point without duplicating seed data.

## Expectations

The `expect` object defines what to validate after a step executes. All checks are optional - only include what's relevant to your test.

### Game State Checks

- `scene_name`: Expected current scene
- `user_location`: Expected player location
- `turn_counter`: Expected global turn count
- `scene_turn_counter`: Expected scene-specific turn count
- `inventory`: Expected items (order-independent, exact match)
- `vars`: Expected variables and their values (only checks specified vars)
- `npc_locations`: Expected NPC positions (e.g., `{"count_dracula": "library"}`)
- `is_ended`: Whether game has ended (`true` or `false`)

### Response Content Checks

- `response_contains`: Array of strings that must appear in response (case-insensitive)
- `response_not_contains`: Array of strings that must NOT appear (case-insensitive)
- `response_regex`: Regex pattern the response must match
- `response_min_length`: Minimum character count
- `response_max_length`: Maximum character count

### Validation Logic

- **Inventory**: Checks for exact match (all expected items present, no extra items)
- **Vars**: Only validates specified variables (other vars can exist)
- **Response**: All `response_contains` entries must be found (partial matching)

## Best Practices

### 1. Keep Tests Story-Focused

**❌ DON'T lead the LLM or reveal test purpose:**
```json
{
  "scenario": "test_scenario.json",
  "chat_history": [
    {
      "role": "assistant",
      "content": "This is a test scenario to validate turn counters."
    }
  ]
}
```

**✅ DO keep tests immersive and story-only:**
```json
{
  "scenario": "space-disaster.json",
  "chat_history": [
    {
      "role": "assistant",
      "content": "The oxygen recyclers are struggling. Warning indicators flash urgently. Time is critical."
    }
  ]
}
```

**Why?** If the LLM knows it's in a test, it may behave differently than in real gameplay, invalidating your test results.

### 2. Test What Matters

**❌ DON'T add unnecessary expectations:**
```json
{
  "name": "Test scene transition",
  "expect": {
    "scene_name": "next_scene",
    "response_contains": [
      "darkness",
      "echo",
      "footsteps",
      "stone",
      "cold",
      "shadow"
    ]
  }
}
```

**✅ DO focus on the core assertion:**
```json
{
  "name": "Test scene transition",
  "expect": {
    "scene_name": "next_scene"
  }
}
```

**Why?** Testing scene transitions shouldn't fail just because the LLM didn't use specific descriptive words. Test the mechanic, not the narrative style.

### 3. Keep Names Concise

**❌ DON'T write verbose names:**
```json
{
  "name": "This is a test to verify that when the player opens the grimoire the conditional prompt fires and Dracula appears"
}
```

**✅ DO write brief, descriptive names:**
```json
{
  "name": "Trigger Dracula appearance prompt"
}
```

**Why?** Test names appear in logs and reports. Keep them scannable.

### 4. Write Natural User Prompts

**❌ DON'T use test-like commands:**
```json
{
  "user_prompt": "Test moving to Castle Gates location"
}
```

**✅ DO write in-character actions:**
```json
{
  "user_prompt": "I walk cautiously through the mist toward the castle gates."
}
```

**Why?** Tests should simulate real gameplay. Players don't say "test moving to X" - they describe actions naturally.

### 5. Provide Sufficient Context

**❌ DON'T start with no context:**
```json
{
  "seed_game_state": {
    "scene_name": "arrival",
    "user_location": "library",
    "turn_counter": 0,
    "scene_turn_counter": 0,
    "chat_history": []
  }
}
```

**✅ DO establish narrative foundation:**
```json
{
  "seed_game_state": {
    "scene_name": "arrival",
    "user_location": "library",
    "turn_counter": 2,
    "scene_turn_counter": 2,
    "chat_history": [
      {
        "role": "user",
        "content": "I enter the castle."
      },
      {
        "role": "assistant",
        "content": "The Grand Foyer envelops you in oppressive darkness..."
      },
      {
        "role": "user",
        "content": "I go to the library."
      },
      {
        "role": "assistant",
        "content": "Ancient tomes line the towering bookshelves..."
      }
    ]
  }
}
```

**Why?** The LLM performs better with context. Show how the player arrived at this state. Simulate a real game in progress.

### 6. One Test, One Purpose

Each test file should validate **one specific feature or behavior**:

- ✅ `dracula_prompt_trigger.json` - Tests variable-based conditional prompt
- ✅ `dracula_wolves_location.json` - Tests location + min turns conditional
- ✅ `space_turn_counter.json` - Tests exact turn counter transitions
- ✅ `pirate_shipwright_vars.json` - Tests variable setting during gameplay

Use rollup files to group related tests:

```json
{
  "name": "Dracula Conditionals - All Tests",
  "cases": [
    "dracula_prompt_trigger.json",
    "dracula_wolves_location.json",
    "dracula_lightning_exact.json"
  ]
}
```

## Common Patterns

### Testing Variable Changes

```json
{
  "name": "Set variable through action",
  "user_prompt": "I open the ancient grimoire.",
  "expect": {
    "vars": {
      "opened_grimoire": "true"
    }
  }
}
```

### Testing Scene Transitions

```json
{
  "name": "Transition to next scene",
  "user_prompt": "I continue my journey.",
  "expect": {
    "scene_name": "next_scene",
    "scene_turn_counter": 0
  }
}
```

Note: `scene_turn_counter` resets to 0 when scenes change.

### Testing Conditional Prompts

```json
{
  "name": "Conditional prompt fires",
  "user_prompt": "I look around carefully.",
  "expect": {
    "response_contains": ["wolves", "yellow eyes"]
  }
}
```

### Testing Inventory Changes

```json
{
  "name": "Pick up item",
  "user_prompt": "I take the ancient key from the pedestal.",
  "expect": {
    "inventory": ["wooden stakes", "silver cross", "ancient key"]
  }
}
```

### Testing Multiple Runs from Same State

```json
{
  "steps": [
    {
      "name": "Try path A",
      "user_prompt": "I go north.",
      "expect": {"user_location": "north_hall"}
    },
    {
      "name": "Reset to beginning",
      "user_prompt": "RESET_GAMESTATE",
      "expect": {"user_location": "grand_foyer"}
    },
    {
      "name": "Try path B",
      "user_prompt": "I go south.",
      "expect": {"user_location": "dungeon"}
    }
  ]
}
```

## Running Tests

### Run all tests:
```bash
go test -tags=integration ./integration/... -v
```

### Run specific test case:
```bash
go test -tags=integration ./integration/... -v -case "dracula_prompt_trigger.json"
```

### Run multiple specific tests:
```bash
go test -tags=integration ./integration/... -v -case "test1.json,test2.json,test3.json"
```

### Run with different error handling:
```bash
# Stop on first failure
go test -tags=integration ./integration/... -v -case "my_test.json" -err exit

# Continue through all steps (default)
go test -tags=integration ./integration/... -v -case "my_test.json" -err continue
```

### Override scenario:
```bash
go test -tags=integration ./integration/... -v -case "test.json" -scenario "pirate.vars.json"
```

## File Naming Conventions

- **Feature tests**: `feature_aspect.json` (e.g., `dracula_prompt_trigger.json`)
- **Rollups**: `category_all.json` (e.g., `dracula_conditionals_all.json`)
- **Keep names lowercase** with underscores
- **Be descriptive but concise**

## Troubleshooting

### Test Fails Inconsistently

- The LLM may not reliably interpret vague prompts or conditions
- Make contingency rules more explicit with specific verbs and items
- Check if conditional prompts need to be more imperative
- Consider if the test expects too much from narrative generation

### Turn Counters Don't Match

- Remember: counters increment *after* AI response
- Check seed data: `turn_counter` should equal number of chat history pairs
- Verify `scene_turn_counter` resets to 0 on scene transitions

### Response Contains Fails

- `response_contains` is case-insensitive - check for typos
- The LLM may paraphrase - look for core concepts, not exact phrasing
- Consider if you're testing narrative style vs. game mechanics

### Inventory Check Fails

- Inventory checks are **exact match** - all items must match exactly
- Item names are case-sensitive in game state
- Use the exact item names from your scenario definitions

**Testing Tip**: When testing inventory-related mechanics, seed the game state with **only the items relevant to your test**. This prevents test failures due to unrelated items the LLM might add or remove. For example, if testing that picking up a key adds it to inventory, seed with just `["torch", "rope"]` rather than seeding the full default inventory from the scenario. This isolates your test to only verify the specific inventory change you care about.

## Examples

See existing test files for examples:
- `dracula_prompt_trigger.json` - Variable-based conditional prompt
- `dracula_wolves_location.json` - Location + minimum turns conditional
- `space_turn_counter.json` - Exact turn counter scene transition
- `dracula_conditionals_all.json` - Rollup test example
