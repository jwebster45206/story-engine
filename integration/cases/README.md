# Integration test cases

One JSON file per mechanic. The filename without `.json` is the `go test -run` subtest name.

Assert the state the engine records. Seed whatever the gate needs, including turn counters when a scene change depends on them, and do not put those counters in `expect`.

## Scene gates

`space_disaster.json` is the counter-gate fixture. Each case waits once, then waits again:

- `space_scene_counter` — `scene_turn_counter` 3 moves `scene_counter` to `turn_counter`
- `space_turn_counter` — `turn_counter` 5 moves `turn_counter` to `min_scene_turns`
- `space_min_scene_turns` — `min_scene_turns` 2 moves `min_scene_turns` to `min_turns`
- `space_min_turns` — `min_turns` 10 sets `is_ended`

The first step asserts the scene has not changed. The second asserts the next scene, or `is_ended` for the last gate.

`pirate_scene1` is the var-driven scene change: paying the shipwright's deposit sets `shipwright_hired` and the scene becomes `british_docks`.

`dracula_scene_cascade` enters the secret passage, which loads the confrontation scene, and then checks the authored story event for `black coffin's lid`.

## Other cases

- `pirate_locations` — movement along exits
- `pirate_items_get_ledger` / `pirate_items_give_ledger` — one phrasing each. Giving the ledger is backed by the shipwright conditional that drops it once `ship_repair_ledger_acquired` is set at the Sleepy Mermaid.
- `pirate_npc_events` — with `davey_recruited` already set, Davey follows a move.
- `pirate_scene_end` — taking the open sea exit ends the game
- `pirate_combat` — attack the cellar skeleton; the runner requires two combat attempt events and the strike-back story event

## Running

```bash
go test -tags=integration ./integration/ -run 'TestIntegration/pirate_combat'
```
