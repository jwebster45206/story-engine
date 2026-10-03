# Integration Tests

Integration tests for the Story Engine that run against a live API and LLM.

## Quick Start

Start the Story Engine API with the scenarios directory loaded (defaults to `http://localhost:8080`), then:

```bash
# Full suite (each JSON case once)
go test -tags=integration ./integration/

# One case (subtest name = filename without .json)
go test -tags=integration ./integration/ -run 'TestIntegration/pirate_scene1'

# Related group (filename prefix)
go test -tags=integration ./integration/ -run 'TestIntegration/space_'
```

These tests are behind `//go:build integration` and are not run by `go test ./...` without the tag.

The server people usually run loads `~/Documents/story-engine-scenarios`, not `data/`.

## What the suite checks

Each case is one mechanic. Assertions are game state, not turn counts and not narrator wording.

- Movement, inventory, scene changes, NPC location, and game end
- Space-disaster scene gates (stay in the scene, then the gate opens)
- A Dracula story event, checked against the authored prompt
- A cellar combat roll and the skeleton's strike-back, checked as SSE `attempt` events

## Test case format

```json
{
  "name": "Pick up the ship repair ledger",
  "scenario": "pirate.json",
  "seed_game_state": {
    "scene_name": "shipwright",
    "user_location": "black_pearl"
  },
  "steps": [
    {
      "name": "Read the ledger",
      "user_prompt": "I pick up the ship repair ledger and read it.",
      "expect": {
        "inventory_contains": ["ship repair ledger"]
      }
    }
  ]
}
```

`seed_game_state` uses gamestate field names. Unknown keys fail the load.

`user_prompt` of `RESET_GAMESTATE` restores the seed. `WAIT_FOR_STORY_EVENT` waits for the next queued story event.

### Expectation fields

| Field | Checks |
| --- | --- |
| `user_location` | Player location id |
| `scene_name` | Current scene id |
| `inventory_contains` | Each item is in inventory |
| `inventory_not_contains` | Each item is absent |
| `vars` | Named variables equal these values |
| `npc_locations` | Named NPCs are at these location ids |
| `is_ended` | Game over flag |
| `combat_roll` | A combat `attempt` on this turn and on the strike-back story event |
| `story_event_contains` | Authored story-event text contains these phrases |

Do not assert `turn_counter` or `scene_turn_counter`. Hit, miss, damage, and hit points are not integration assertions.

## Configuration

| Flag | Default | Description |
| --- | --- | --- |
| `-scenario` | "" | Override scenario for all cases |
| `-err` | `continue` | `continue` runs remaining steps; `exit` stops the case on the first failure |

| Variable | Default | Description |
| --- | --- | --- |
| `API_BASE_URL` | `http://localhost:8080` | API to test |
| `TEST_TIMEOUT_SECONDS` | `30` | Timeout per step |

## Flow

1. `POST /v1/gamestate` creates the session.
2. `PATCH /v1/gamestate/{id}` applies the seed.
3. `POST /v1/chat` returns `202` and a request id.
4. The runner polls `GET /v1/gamestate/{id}` until the turn is applied.
5. A `combat_roll` step also listens on `GET /v1/events/gamestate/{id}`.

Cases run sequentially. Each case gets its own gamestate.

PATCH replaces inventory, vars, NPCs, and chat history only when the seeded value is non-empty. An empty inventory list does not clear the PC's starting gear, so item checks use contains and not-contains.
