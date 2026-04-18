---
name: story-engine-transition
description: "Write, review, debug, or improve scene transitions in story-engine scenario JSON files. Use when: adding a scene change, a transition is not firing, a conditional is misconfigured, the recommended transition pattern is needed, vars are not triggering a scene change, scene_turn_counter vs min_scene_turns confusion, cascading conditionals, or auditing an existing transition. Trigger words: scene change, transition, scene_change, conditional, scene not firing, scene progression, next scene, story-engine-transition, vars and conditionals."
argument-hint: "create <goal> | review | debug | cascade | audit-all"
---

# story-engine-transition

Write, review, and debug scene transitions in story-engine scenario JSON files. Ensures transitions use vars + conditionals as the primary pattern, correct conditional syntax, distinct var names, and proper scene wiring.

## When to Use

- **Create**: Wire a new scene transition for a specific gameplay goal
- **Review**: Audit existing transitions against the checklist below
- **Debug**: A `scene_change` conditional is not firing
- **Cascade**: Build a multi-step initialization chain when a scene loads
- **Audit all**: Scan every scene in a scenario for transition correctness

---

## How Scene Transitions Work

A transition moves the player from one scene to another. The engine evaluates all conditionals in the **current scene** every turn. When a conditional's `when` clause passes, the `then` block executes. If `then` contains `scene_change`, the engine calls `LoadScene()` which:

1. Sets `SceneName` to the target scene
2. **Resets `SceneTurnCounter` to 0** — critical for `scene_turn_counter` and `min_scene_turns` timing
3. **Merges** scene vars into `GameState.Vars` — old vars persist unless explicitly overwritten
4. **Merges** scene locations and NPCs over the global set — scene-only locations/NPCs are scoped to the scene
5. Re-evaluates conditionals immediately (enabling cascades)

### Key Runtime Facts

| Fact | Detail |
|---|---|
| Vars are `map[string]string` | All values are strings. No booleans, no integers, no operators. |
| All `when` fields are AND logic | Every field in `when` must pass for the conditional to fire |
| `scene_turn_counter` is **exact match** | Fires only on that one turn. If the turn passes, it never fires again. |
| `min_scene_turns` is **>= match** | Fires on that turn and every subsequent turn. Usually what you want. |
| `turn_counter` / `min_turns` | Same pattern but for the global (cross-scene) turn counter |
| `location` | Exact string match against the player's current location key |
| Vars persist across scenes | Scene vars merge in; they do not replace the whole map |
| `SceneTurnCounter` resets on scene load | Always starts at 0 in the new scene |
| Empty `when` never fires | A conditional with `when: {}` evaluates to `false` |

---

## The Recommended Transition Pattern

Every scene transition should use all three of these elements. Vars and conditionals are the core — they provide the hard guarantee. Contingency rules and prompts support them.

### Layer 1 — Contingency prompts (narrative guidance)

Tell the narrator *what conditions the player should work toward* and *what the scene goal is*. These are soft hints — the LLM may ignore them.

```json
"contingency_prompts": [
  "The shipwright is agreeable to most forms of downpayment and ready to start work quickly."
]
```

### Layer 2 — Contingency rules (state changes)

Tell the game engine *what state changes to make* when the triggering event occurs. One rule sets the var; a separate rule names the scene change. Keep them separate — the LLM processes rules independently, and splitting them means the var gets set even if the scene change wording is missed.

```json
"contingency_rules": [
  "When the shipwright agrees to start repairs, set the variable \"shipwright_hired\" to \"true\".",
  "When the shipwright starts repairs, the scene changes to 'british_docks'."
]
```

### Layer 3 — Conditional (deterministic enforcement)

The hard guarantee. If the var is set, the engine forces the transition regardless of what the LLM did. **This is the most important layer** — the conditional catches any turn the LLM failed to transition on its own.

```json
"conditionals": {
  "hire_shipwright_transition": {
    "when": {
      "vars": { "shipwright_hired": "true" }
    },
    "then": {
      "scene_change": {
        "to": "british_docks",
        "reason": "conditional"
      }
    }
  }
}
```

### The full wiring for one transition

Putting it all together in the scene JSON:

```json
"shipwright": {
  "story": "The player must find the shipwright and hire him to repair the Black Pearl.",
  "vars": {
    "shipwright_hired": "false"
  },
  "contingency_prompts": [
    "The shipwright is agreeable to most forms of downpayment and ready to start work quickly."
  ],
  "contingency_rules": [
    "When the shipwright agrees to start repairs, set the variable \"shipwright_hired\" to \"true\".",
    "When the shipwright starts repairs, the scene changes to 'british_docks'."
  ],
  "conditionals": {
    "hire_shipwright_transition": {
      "when": {
        "vars": { "shipwright_hired": "true" }
      },
      "then": {
        "scene_change": {
          "to": "british_docks",
          "reason": "conditional"
        }
      }
    }
  }
}
```

---

## Conditional Syntax Reference

### `when` fields

All fields are AND logic. Every specified field must pass.

| Field | Type | Match | Use for |
|---|---|---|---|
| `vars` | `map[string]string` | Exact per key, AND across keys | Most transitions — var-gated progression |
| `scene_turn_counter` | `int` | Exact `==` | One-shot timed events (fires once, on that exact turn) |
| `turn_counter` | `int` | Exact `==` | One-shot global timed events |
| `min_scene_turns` | `int` | `>=` | Time-pressure gates (fires on that turn and every turn after) |
| `min_turns` | `int` | `>=` | Global time-pressure gates |
| `location` | `string` | Exact location key | Location-triggered transitions |

### `then` fields

Multiple fields can be combined in one `then` block. All execute together.

| Field | Type | Effect |
|---|---|---|
| `scene_change` | `{ "to": "scene_id", "reason": "..." }` | Transition to another scene |
| `set_vars` | `map[string]string` | Update game vars |
| `user_location` | `string` | Move the player to a location |
| `prompt` | `string` | Inject a narrative prompt into the story as a plain user-side message |
| `game_ended` | `bool` | End the game |
| `npc_events` | `[]object` | Move NPCs, set following behavior |
| `monster_events` | `[]object` | Spawn or despawn monsters |
| `item_events` | `[]object` | Move items between player/NPC/location |

### Common `when` examples

**Var-gated (most common):**
```json
"when": { "vars": { "shipwright_hired": "true" } }
```

**Multi-var gate (AND logic — all must match):**
```json
"when": {
  "vars": {
    "gold_acquired": "true",
    "shipwright_paid_in_full": "true"
  }
}
```

**Location-triggered:**
```json
"when": { "location": "secret_passage" }
```

**Turn-gated (minimum — preferred):**
```json
"when": { "min_scene_turns": 2 }
```

**Turn-gated (exact — use sparingly):**
```json
"when": { "scene_turn_counter": 3 }
```

**Combined var + location:**
```json
"when": {
  "vars": { "has_key": "true" },
  "location": "locked_door"
}
```

---

## Scene Initialization with `scene_turn_counter: 0`

When a scene loads, `SceneTurnCounter` resets to 0. A conditional with `"scene_turn_counter": 0` fires **immediately** before the player's first action.

Use this to:
- Force the player to a specific starting location
- Set vars that trigger cascading conditionals
- Fire an opening story event

### Cascade example (from Dracula)

Scene `confrontation` loads. Three conditionals fire in sequence:

**Step 1 — Initialization (turn 0):**
```json
"enter_sanctum": {
  "when": { "scene_turn_counter": 0 },
  "then": {
    "user_location": "draculas_sanctum",
    "set_vars": { "entered_sanctum": "true" }
  }
}
```

**Step 2 — Cascading story event (triggered by var set in step 1):**
```json
"dracula_rises": {
  "when": { "vars": { "entered_sanctum": "true" } },
  "then": {
    "prompt": "Count Dracula rises from his coffin, eyes blazing with unholy power."
  }
}
```

**How this works:**
1. Scene loads → `SceneTurnCounter` = 0
2. `enter_sanctum` fires → player moves to sanctum, `entered_sanctum` = `"true"`
3. Engine re-evaluates → `dracula_rises` fires → story event injected
4. Player takes their first action in the scene (counter increments to 1)

---

## Procedures

### 1. Create a Transition

Goal: wire a new scene change with all three layers.

1. **Define the trigger**: What player action or game event should cause the transition?
   - Player completes an objective → var-gated
   - Player reaches a specific location → location-triggered
   - Enough time has passed → turn-gated (`min_scene_turns`)
   - Combination of the above

2. **Verify the target scene exists**: Confirm the target scene ID exists as a key in `scenario.scenes`. Typos here fail silently.

3. **Create the var** (if var-gated):
   - Pick a descriptive name: `<subject>_<state>` (e.g., `shipwright_hired`, `treasure_map_acquired`)
   - Initialize in the current scene's `vars` block with value `"false"`
   - **Also declare in the scenario-level `vars` block** if one exists (ensures the key exists from game start)

4. **Write Layer 1** — Add a contingency prompt hinting at the goal:
   ```json
   "contingency_prompts": [
     "The shipwright needs a small deposit to begin repairs."
   ]
   ```

5. **Write Layer 2** — Add contingency rules for the var and the scene change:
   ```json
   "contingency_rules": [
     "When the shipwright agrees to start repairs, set the variable \"shipwright_hired\" to \"true\".",
     "When the shipwright starts repairs, the scene changes to 'british_docks'."
   ]
   ```
   Rules should be **explicit and action-focused** — list specific player verbs and exit names. Keep the var-setter and scene-change as separate rules.

6. **Write Layer 3** — Add the conditional:
   ```json
   "conditionals": {
     "hire_shipwright_transition": {
       "when": { "vars": { "shipwright_hired": "true" } },
       "then": { "scene_change": { "to": "british_docks", "reason": "conditional" } }
     }
   }
   ```

7. **Validate** using the checklist below.

### 2. Review Existing Transitions

1. For each scene, list all conditionals that contain `scene_change`.
2. For each transition conditional:
   - Does a contingency rule exist that sets the triggering var? (Layer 2)
   - Does a contingency rule exist that names the scene change? (Layer 2)
   - Does a narrative contingency prompt exist? (Layer 1)
3. For each var referenced in transition `when.vars`:
   - Is it initialized in the scene `vars` block?
   - Is it also declared in scenario-level `vars` (if present)?
   - Does at least one contingency rule set it?
   - Is the expected value an exact string match? (case-sensitive, no whitespace)
4. For the target scene ID in `then.scene_change.to`:
   - Does the target scene exist as a key in `scenario.scenes`?
5. Report findings. Flag missing layers, undeclared vars, and typos.

### 3. Debug a Transition That Won't Fire

Work through this checklist in order:

**Step 1 — Find the conditional.**
Locate the `scene_change` conditional. Confirm the conditional ID exists in the current scene's `conditionals` map.

**Step 2 — Check `then.scene_change.to`.**
Confirm the target scene ID exactly matches a key in `scenario.scenes`. Case-sensitive. No trailing spaces.

**Step 3 — Check every `when` field.**
For `when.vars`:
- Is each var key initialized in the scene's `vars` block?
- Is each var key declared at scenario-level `vars`?
- Does a contingency rule set it to the **exact** expected value? (exact string — `"true"` not `"True"`, `"yes"`, or `true`)
- Has a prior scene's `vars` block overwritten the value back to its initial state?

For `when.scene_turn_counter` / `when.turn_counter`:
- Is the turn window still reachable? Exact-match counters fire once — if the turn passed, it never fires again.
- Consider switching to `min_scene_turns` / `min_turns` if the window is too narrow.

For `when.location`:
- Does the location key match an actual location in the current scene's resolved location map?

**Step 4 — Check the setter.**
Is there a contingency rule that sets the triggering var to the expected value? If no rule tells the LLM to set the var, it will never be set.

**Step 5 — Check for var shadowing.**
Does the target scene (or any intermediate scene) re-initialize the var in its `vars` block, resetting it to `"false"` and immediately un-triggering the conditional?

**Step 6 — Check contingency rule wording.**
Is the rule specific enough? Vague rules like "When the player helps the NPC" are unreliable. Rules should list concrete player verbs and item/NPC names.

**Common root causes:**

| Symptom | Likely cause |
|---|---|
| Conditional never fires | Var never set — missing contingency rule or rule wording too vague |
| Conditional fires too early | Var initialized to `"true"` instead of `"false"` in scene vars |
| Fires once then stops on reload | `scene_turn_counter` exact match — missed the window |
| Target scene loads but immediately transitions again | Target scene vars overwrite a var that triggers another conditional |
| Scene change fires but player is in wrong location | Missing `user_location` in `then` block — add scene initialization |
| Cascade doesn't complete | Intermediate var not set, or `when` order creates a circular dependency |

### 4. Build a Cascade

Goal: when a scene loads, fire a chain of conditionals in sequence.

1. **Start with scene initialization** — use `"scene_turn_counter": 0` to fire immediately on load:
   ```json
   "init": {
     "when": { "scene_turn_counter": 0 },
     "then": {
       "user_location": "target_location",
       "set_vars": { "scene_initialized": "true" }
     }
   }
   ```

2. **Chain with vars** — each subsequent conditional triggers off a var set by the previous one:
   ```json
   "step_two": {
     "when": { "vars": { "scene_initialized": "true" } },
     "then": {
       "prompt": "Something dramatic happens.",
       "set_vars": { "drama_happened": "true" }
     }
   }
   ```

3. **Avoid circular chains** — never set a var that a prior conditional in the chain reads. The engine evaluates all conditionals per turn; a circular dependency causes unpredictable behavior.

4. **Keep cascades short** — 2–3 steps maximum. Deeper chains are fragile and hard to debug.

---

## Transition Trigger Patterns

### Pattern A: Var-Gated (most common)

Player completes an objective → var is set → conditional fires scene change.

**Best for:** Quest completion, NPC interactions, item acquisition, puzzle solving.

```json
"vars": { "shipwright_hired": "false" },
"contingency_rules": [
  "When the shipwright agrees to begin repairs, set \"shipwright_hired\" to \"true\"."
],
"conditionals": {
  "to_british_docks": {
    "when": { "vars": { "shipwright_hired": "true" } },
    "then": { "scene_change": { "to": "british_docks", "reason": "conditional" } }
  }
}
```

### Pattern B: Location-Triggered

Player enters a specific location → conditional fires immediately.

```json
"when": { "location": "secret_passage" },
"then": { "scene_change": { "to": "confrontation", "reason": "entered via secret passage" } }
```

**Warning:** Location-triggered transitions fire every turn the player is at that location. If the target scene also has that location, add a var guard to prevent re-triggering.

### Pattern C: Time-Pressure

Enough turns pass → conditional fires. **Prefer `min_scene_turns` over `scene_turn_counter`** — it fires on that turn and all subsequent turns, so it can't be missed.

```json
"when": { "min_scene_turns": 10 },
"then": { "game_ended": true }
```

### Pattern D: Combined (var + location or var + time)

All `when` fields use AND logic — every field must pass.

```json
"when": {
  "vars": { "has_key": "true" },
  "location": "vault_door"
},
"then": { "scene_change": { "to": "vault", "reason": "conditional" } }
```

### Pattern E: Game End

Same structure, but `then` uses `game_ended` instead of `scene_change`:

```json
"when": { "vars": { "pearl_departed_tortuga": "true" } },
"then": { "game_ended": true }
```

---

## Review Checklist

Before finalizing any transition:

- [ ] **Three layers present** — contingency prompt + contingency rules + conditional
- [ ] **Var initialized** — triggering var exists in scene `vars` with initial value `"false"` (or appropriate default)
- [ ] **Var declared at scenario level** — if scenario has a top-level `vars` block, the var is also declared there
- [ ] **Var has a setter** — a contingency rule explicitly sets the var to the expected value
- [ ] **Setter rule is specific** — lists concrete player verbs and names, not abstract conditions
- [ ] **Scene change rule present** — a separate contingency rule names the scene change
- [ ] **Target scene exists** — `then.scene_change.to` matches an exact key in `scenario.scenes`
- [ ] **No var shadowing** — target scene's `vars` block does not overwrite the triggering var to a value that re-triggers the source conditional
- [ ] **Correct counter type** — uses `min_scene_turns` instead of `scene_turn_counter` unless exact-turn timing is intentional
- [ ] **No empty `when`** — a `when: {}` block never fires; every conditional must have at least one condition
- [ ] **Conditional ID is descriptive** — names like `hire_shipwright_transition` not `cond1`

---

## Anti-Patterns

| Anti-pattern | Problem | Fix |
|---|---|---|
| Missing Layer 2 (no contingency rules) | LLM has no instruction to set the var → conditional never fires | Add a rule to set the var and a separate rule to name the scene change |
| Var initialized to `"true"` | Conditional fires immediately on scene load | Initialize to `"false"` |
| `scene_turn_counter` for progression | Fires once and is easily missed | Use `min_scene_turns` or var-gated instead |
| Same var name across scenes | Scene load merges vars — stale values from prior scenes cause false triggers | Use distinct var names per scene, or explicitly reset vars in the new scene's `vars` block |
| Abstract contingency rules | "When the player succeeds" — too vague for the LLM | "When the player gives 'gold coins' to the shipwright, set..." |
| Combining set_var + scene change in one rule | LLM may partially execute | Split into two separate contingency rules |
| Target scene ID typo | Scene change targets a scene that doesn't exist — fails silently | Double-check the key in `scenario.scenes` |
| Location trigger without var guard | Player enters location → scene changes → new scene has same location → infinite loop | Add a var to track that the transition already happened |
