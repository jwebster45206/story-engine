# Integration Tests

Integration tests for the Story Engine that run against a live API and LLM.

## Quick Start

Start the Story Engine API (defaults to `http://localhost:8080`), then:

```bash
# Full suite (each JSON case once)
go test -v -tags=integration ./integration/

# One case (subtest name = filename without .json)
go test -v -tags=integration ./integration/ -run 'TestIntegration/pirate_scene1'

# Related group (filename prefix)
go test -v -tags=integration ./integration/ -run 'TestIntegration/space_'

# Repeat a case (flake check)
go test -v -tags=integration ./integration/ -run 'TestIntegration/pirate_scene1' -count=3
```

These tests are behind `//go:build integration` and are **not** run by GitHub Actions (`go test ./...` without the tag).

## Overview

These tests validate:
- Real LLM integration
- API endpoint functionality
- Gamestate persistence and updates
- Game mechanics (inventory, location changes, variables)
- Scene transitions and game flow

## Test Structure

### Test Files
- `cases/` — JSON test case definitions (one feature per file)
- `runner/` — Test execution framework
  - `types.go` — Data structures for test definitions
  - `runner.go` — Core test execution logic

### Test Case Format

```json
{
  "name": "Test Name",
  "scenario": "scenario.json",
  "seed_game_state": {
    "provider": "sonnet",
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
        "inventory": ["item1", "new_item"],
        "response_contains": ["expected", "words"],
        "turn_counter": 1
      }
    }
  ]
}
```

See `cases/README.md` for seed/step authoring details, including `RESET_GAMESTATE` and `WAIT_FOR_STORY_EVENT`.

## Configuration

### Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-scenario` | "" | Override scenario for all test cases (e.g. `pirate.json`) |
| `-err` | `continue` | `continue` (run all steps) or `exit` (stop on first failure) |

Selection is stock `go test -run`. Repeat with `go test -count=N`.

### Scenario Override

The `-scenario` flag tests the same JSON cases against different scenario variants without duplicating files:

```bash
go test -v -tags=integration ./integration/ -run 'TestIntegration/pirate_scene1' -scenario pirate.json
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `API_BASE_URL` | `http://localhost:8080` | Base URL of the API to test |
| `TEST_TIMEOUT_SECONDS` | `30` | Timeout per test step in seconds |

```bash
API_BASE_URL=http://api.example.com:8080 go test -v -tags=integration ./integration/
TEST_TIMEOUT_SECONDS=60 go test -v -tags=integration ./integration/
```

## Test Development

1. Create a new JSON file in `integration/cases/`
2. Use an existing scenario from `data/scenarios/`
3. Seed realistic state with minimal chat history
4. Define steps with specific expectations
5. Run that file: `go test -v -tags=integration ./integration/ -run 'TestIntegration/<filename>'`

### Best Practices

- **Realistic chat history**: Include just enough context for the LLM (2–4 messages)
- **Specific expectations**: Test only what matters for each step
- **Scenario consistency**: Use existing scenario data
- **Descriptive names**: Filename (minus `.json`) is the `go test -run` subtest name

### Expectation Types

| Type | Description | Example |
|------|-------------|---------|
| `location` | Exact location match | `"Black Pearl"` |
| `scene_name` | Current scene | `"shipwright"` |
| `inventory` | Full inventory (order-independent) | `["sword"]` |
| `vars` | Variable values | `{"door_open": "true"}` |
| `npc_locations` | NPC positions | `{"Gibbs": "Black Pearl"}` |
| `response_contains` | Required text (case-insensitive) | `["ship", "deck"]` |
| `response_regex` | Regex pattern match | `".*treasure.*map.*"` |
| `is_ended` | Game completion status | `true` |
| `turn_counter` | Turn count | `3` |

## Architecture

### Test Flow
1. **Create**: `POST /v1/gamestate` (immutable scenario and provider)
2. **Seed**: `PATCH /v1/gamestate/{id}` (location, inventory, etc.)
3. **Execute**: `POST /v1/chat` (`202` + `request_id`)
4. **Poll**: Wait for gamestate update via `GET /v1/gamestate/{id}`
5. **Validate**: Check expectations against updated gamestate and response
6. **Repeat** for each step

`provider`, `model_name`, and `scenario` are set on create and cannot be changed via PATCH. Optional seed `provider` is passed on create; omit it to use the server default.

Cases run **sequentially**. Each case gets its own gamestate.

### Error Handling
- `-err continue` (default): remaining steps in a case still run after a failure
- `-err exit`: stop that case on the first failed step
- Per-step timeout via `TEST_TIMEOUT_SECONDS`
