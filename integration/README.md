# Integration Tests

Integration tests for the Story Engine that test against a real running API.

## Quick Start

### Prerequisites
- Start the Story Engine API (defaults to `http://localhost:8080`)

### Run All Integration Tests
```bash
go test -v -tags=integration ./integration/
```

### Run a Specific Test Case
```bash
# Run by case name (automatically adds .json extension and cases/ path)
go test -v -tags=integration ./integration/ -run TestSingleSuite -case pirate_shipwright_to_british_docks

# Other examples:
go test -v -tags=integration ./integration/ -run TestSingleSuite -case pirate_british_docks_to_calypsos_map
go test -v -tags=integration ./integration/ -run TestSingleSuite -case pirate_basic_movement
```

## Overview

These tests validate:
- Real LLM integration 
- API endpoint functionality 
- Gamestate persistence and updates
- Game mechanics (inventory, location changes, variables)
- Scene transitions and game flow

## Test Structure

### Test Files
- `cases/` - JSON test case definitions
- `runner/` - Test execution framework
  - `types.go` - Data structures for test definitions
  - `runner.go` - Core test execution logic

### Test Case Format

```json
{
  "name": "Test Name",
  "scenario": "scenario.json",
  "seed_game_state": {
    "model_name": "claude-3-5-sonnet-20241022",
    "scenario": "scenario.json",
    "location": "Starting Location",
    "turn_counter": 0,
    "inventory": ["item1", "item2"],
    "vars": {
      "some_flag": "true"
    },
    "chat_history": [
      {
        "role": "user",
        "content": "Previous user message"
      },
      {
        "role": "assistant",
        "content": "Previous assistant response"
      }
    ]
  },
  "steps": [
    {
      "name": "Step Name",
      "user_prompt": "What the user types",
      "expect": {
        "location": "Expected Location",
        "inventory_added": ["new_item"],
        "response_contains": ["expected", "words"],
        "turn_increment": 1
      }
    }
  ]
}
```

## Configuration

### API Base URL
Default: `http://localhost:8080`
Override with environment variable:
```bash
API_BASE_URL=http://api.example.com:8080 go test -v -tags=integration ./integration/
```

### Test Timeout  
Default: 30 seconds per test step
Override with environment variable:
```bash
TEST_TIMEOUT_SECONDS=60 go test -v -tags=integration ./integration/
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `API_BASE_URL` | `http://story-engine-api:8080` | Base URL of the API to test |
| `TEST_TIMEOUT_SECONDS` | `30` | Timeout per test step in seconds |

## Test Development

### Adding New Tests

1. Create a new JSON file in `integration/cases/`
2. Define your test scenario using existing scenarios from `data/scenarios/`
3. Set up realistic seed state with minimal chat history
4. Define test steps with specific expectations
5. Run your test to validate it works

### Best Practices

- **Realistic Chat History**: Include just enough context for the LLM (2-4 messages)
- **Specific Expectations**: Test only what matters for each step
- **Progressive Complexity**: Start with simple movements, build to complex interactions
- **Scenario Consistency**: Use existing scenario data to ensure test validity
- **Descriptive Names**: Use clear step names for debugging

### Expectation Types

| Type | Description | Example |
|------|-------------|---------|
| `location` | Exact location match | `"Black Pearl"` |
| `scene_name` | Current scene | `"shipwright"` |
| `inventory_added` | Items gained this step | `["sword"]` |
| `inventory_removed` | Items lost this step | `["gold"]` |
| `vars` | Variable values | `{"door_open": "true"}` |
| `npc_locations` | NPC positions | `{"Gibbs": "Black Pearl"}` |
| `response_contains` | Required text (case-insensitive) | `["ship", "deck"]` |
| `response_regex` | Regex pattern match | `".*treasure.*map.*"` |
| `game_ended` | Game completion status | `true` |
| `turn_increment` | Turn counter change | `1` |

## Architecture

### Test Flow
1. **Create**: Create new gamestate via `POST /v1/gamestate` (sets immutable scenario and model)
2. **Seed**: Patch gamestate with test data via `PATCH /v1/gamestate/{id}` (location, inventory, etc.)
3. **Execute**: Send user prompt to `/v1/chat?id={gamestate_id}`
4. **Poll**: Wait for gamestate update by polling `/v1/gamestate/{id}`
5. **Validate**: Check expectations against updated gamestate and response
6. **Repeat**: Continue for each test step

**Note**: `ModelName` and `Scenario` are immutable and set during creation - they cannot be changed via PATCH.

### Parallel Execution
- Tests run with configurable concurrency (default: 5)
- Each test gets unique gamestate ID to avoid conflicts
- Simple worker pool pattern with shared rate limiting

### Error Handling
- Individual step failures don't stop the suite
- Detailed error reporting with context
- Timeout protection for hung tests
- Graceful degradation on API issues

## Example Output

```
🏴‍☠️ Running Story Engine Integration Tests
   API Base URL: http://story-engine-api:8080
📋 Loaded 1 test suites
   - Pirate Basic Movement Test (5 steps)
🚀 Running tests with concurrency 5...
✅ Test suite 'Pirate Basic Movement Test' passed in 45.2s
   ✓ Move to Black Pearl (8.1s)
   ✓ Look around ship deck (6.3s)
   ✓ Pick up ship repair ledger (7.8s)
   ✓ Go to Captain's Cabin (9.2s)
   ✓ Return to deck (6.1s)

📊 Integration Test Summary:
   Passed: 1
   Failed: 0

🎉 All integration tests passed!
```