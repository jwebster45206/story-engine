---
name: story-engine-vars
description: "Audit, minify, repair, or surgically insert vars and contingency_prompts in story-engine scenario JSON files. Use when: tuning scenario vars, removing unused vars, vars are bloated, scene changes are not triggering, contingency prompts never appear, conditionals fail silently, or you need a new gameplay flag. Trigger words: vars, contingency_prompts, scene change failing, conditional not firing, game flag."
argument-hint: "audit | minify | remove-all | add-var <goal> | troubleshoot | explain"
---

# story-engine-vars

Audit, minify, and surgically modify `vars` and `contingency_prompts` across story-engine scenario JSON files. Also diagnoses why scene changes and conditionals fail.

## When to Use

- **Audit / minify**: Scenario has too many vars; want to identify and remove dead weight
- **Remove all**: Strip every var from a scenario for a clean slate
- **Add var**: Need a new flag to gate a gameplay event, scene transition, item event, or NPC event
- **Troubleshoot**: A `conditional` or `scene_change` is not firing; a `contingency_prompt` is never appearing

---

## Key Concepts

### Vars — ground truth

Vars are `map[string]string` stored at **scenario**, **scene**, and **runtime** (`GameState.Vars`) levels. All values are strings. There are no typed fields, no operators beyond exact-match.

| Level | Where defined | Lifecycle |
|---|---|---|
| Scenario | `scenario.vars` | Set at game load; persist for entire game |
| Scene | `scene.vars` | Merged/overridden on `LoadScene()`; old vars NOT cleared |
| Runtime | Set via `conditional.then.set_vars` | Survive across scene transitions |

Scene vars are **merged-in**, they do not replace the whole map. A var set in Scene A persists into Scene B unless Scene B explicitly resets it.

**Every var must be declared at scenario level.** If a var is only written by `contingency_rules` at runtime and not pre-declared in `scenario.vars`, it does not exist in `GameState.Vars` until the LLM first writes it. Any `when.vars` check against an undeclared var will behave unpredictably. Always add an initializer entry for every var in the top-level `"vars"` block of the scenario, even if the initial value is never read by a conditional.

Use `"false"` for boolean flags. Use a descriptive string like `"none"` for multi-state vars that progress through named states (e.g., `"none"` → `"started"` → `"secure"`). Do not initialize multi-state vars to `"false"` — it is misleading and creates an extra state that is never set.

### Conditionals — deterministic event triggers

```json
"conditionals": {
  "my_event_id": {
    "when": { "vars": { "flag": "true" }, "min_scene_turns": 1 },
    "then": {
      "scene_change": { "to": "next_scene", "reason": "conditional" },
      "set_vars": { "flag": "used" },
      "prompt": "STORY EVENT: ..."
    }
  }
}
```

All `when` fields are **AND logic**. Supported `when` fields:

| Field | Match type |
|---|---|
| `vars` | Exact string match per key (AND across keys) |
| `scene_turn_counter` | Exact integer |
| `turn_counter` | Exact integer |
| `min_scene_turns` | Integer >= |
| `min_turns` | Integer >= |
| `location` | Exact location key string |

### ContingencyPrompt

Injected into every LLM prompt when the `when` conditions pass. Supports plain strings or conditional objects:

```json
"contingency_prompts": [
  "Always show this.",
  {
    "prompt": "Only when flag is true.",
    "when": { "vars": { "flag": "true" } }
  }
]
```

Contingency prompts exist at five levels: scenario, PC, scene, NPC, location. All active ones are merged for each turn.

---

## Procedures

### 1. Audit Vars

Goal: find unused, redundant, or over-specified vars.

1. Read the target JSON file(s). Identify every key in all `vars` maps (scenario + each scene).
2. Confirm every var is declared in the top-level `scenario.vars` block. Any var that exists only in a scene `vars` block or is only ever written at runtime by `set_vars`/`contingency_rules` but has no top-level declaration is **undeclared** and must be added.
3. For each var key, search the file for all uses:
   - `conditional.when.vars` — is this key checked anywhere?
   - `conditional.then.set_vars` — is this key ever set?
   - `contingency_prompt.when.vars` — is this key read by any prompt?
3. Classify each var:
   - **Dead** — defined but never checked
   - **Write-only** — set but never checked; delete or collapse to a single check
   - **Redundant** — two vars track the same state
   - **Over-specified** — uses `scene_turn_counter: 2` when `min_scene_turns: 2` is equivalent and more robust
4. Report findings before making changes. Confirm before deleting.

### 2. Minify Vars

Goal: remove dead vars, collapse redundant ones, and simplify `when` conditions.

1. Run the Audit (above).
2. For dead vars: remove from every `vars` map and from any `set_vars` block that sets them.
3. For write-only vars: trace all `set_vars` writes; if nothing reads the var, delete all writes.
4. For redundant vars: pick the single surviving name, update all `when.vars`, `set_vars`, and `contingency_prompt.when.vars` to use it, then delete the duplicate.
5. Prefer `min_scene_turns` over exact `scene_turn_counter` unless the exact turn matters.
6. Prefer `min_turns` over `turn_counter` for time pressure gates.
7. Validate: every remaining var must have at least one writer (`set_vars` or scene `vars` init) and at least one reader (`when.vars` or `contingency_prompt.when.vars`).

### 3. Remove All Vars

Goal: strip the scenario to zero vars.

1. Confirm with user before proceeding.
2. Delete all `vars` fields at scenario and scene levels.
3. Delete all `set_vars` fields inside `conditional.then` blocks.
4. Delete all `when.vars` checks inside `conditionals` and `contingency_prompts`.
5. If a `when` block is now empty `{}`, delete the entire `when` key (making the contingency always-on).
6. If a `conditional` has an empty `when: {}`, decide with user whether to:
   - Fire unconditionally — remove the `when` entirely, or
   - Delete the conditional entirely.
7. Validate: no `vars`, `set_vars`, or `when.vars` references remain.

### 4. Add Var (Surgical Insertion)

Goal: add a minimal var to accomplish a specific gameplay goal.

1. Clarify the goal:
   - What event sets the flag? (player action, item acquired, NPC event, turn threshold)
   - What does the flag gate? (scene change, monster spawn/despawn, NPC move, item event, prompt change)
   - Should the flag reset between scenes, or persist?
2. Pick the narrowest var name: `<subject>_<state>` (e.g., `gate_unlocked`, `boss_defeated`, `npc_met`).
3. Initialize the var in the appropriate scope:
   - Scene-level `vars` if it resets per scene
   - Scenario-level `vars` if it must persist across scenes
   - **Always also add the var to the top-level `scenario.vars` block** with its default value, even for scene-scoped vars. This ensures the key exists in `GameState.Vars` from turn one and `when.vars` checks behave predictably.
4. Add a `set_vars` in the `then` block of the conditional that detects the triggering event.
5. Add a `when.vars` check on the conditional that gates the outcome.
6. If the flag should be visible to the LLM, add or update a `contingency_prompt` that activates `when` the flag is set.
7. Test pattern: verify the new var has exactly one initializer, at least one setter, and at least one reader.

### 5. Troubleshoot Scene Changes Not Firing

Work through this checklist in order:

**Step 1 — Find the conditional**
- Locate the `scene_change` conditional. Confirm `then.scene_change.to` matches an actual scene key in `scenario.scenes`.

**Step 2 — Check the `when` fields**
- List every field in `when`. For `vars`, confirm:
  - The var key exists in `GameState.Vars` at the time the conditional is evaluated (check scenario/scene `vars` initializers and any prior `set_vars`).
  - The expected string value matches exactly (case-sensitive, no whitespace).
- For `scene_turn_counter` / `turn_counter`: confirm it's an exact match, not a >=. If the turn has already passed, it will never match again. Switch to `min_scene_turns` / `min_turns` if the window shouldn't be this narrow.

**Step 3 — Check for setter**
- Is `set_vars` for the triggering var present in a `then` block that actually fires? If the var is never set to the expected value, the conditional never passes.

**Step 4 — Check for shadowing**
- Is a later scene overriding the var back to its initial value on `LoadScene()`? Check scene-level `vars` in subsequent scenes.

**Step 5 — Check `contingency_prompts`**
- Is the LLM being instructed (via a contingency prompt) to set the var when the right narrative event happens? Without a prompt, the LLM has no mechanism to trigger `set_vars`.
- A contingency prompt like `"When the player does X, the system sets flag_name to 'true'."` tells the LLM to create the delta that writes the var.

**Step 6 — Check target scene**
- Confirm the target scene ID is spelled correctly (case-sensitive) in `then.scene_change.to` and exists as a key in `scenario.scenes`.

**Common root causes summary:**

| Symptom | Likely cause |
|---|---|
| Conditional never fires | Var never set to expected value |
| Scene change fires once then stops | `scene_turn_counter` exact match, window already passed |
| Scene change fires but resets | Target scene `vars` overwrites the flag |
| Contingency prompt never appears | Wrong level (should be at scene, not scenario), or `when.vars` value mismatch |
| LLM ignores var-setting instruction | Contingency prompt absent; LLM has no instruction to write the var |

---

## Minimal Var Rules (defaults to enforce)

1. Maximum ~4–6 vars per scene. More than 8 is a smell.
2. Every var must have a reader and a writer.
3. Boolean flags use `"true"` / `"false"` strings — never `"yes"`, `"no"`, `"1"`, `"0"`.
4. Prefer `min_scene_turns` over exact `scene_turn_counter` unless a precise one-turn gate is needed.
5. Reset a var in scene `vars` if its stale value from a prior scene could cause incorrect conditional matches.
6. Don't use vars to track LLM narrative state — only track deterministic game-state changes.
