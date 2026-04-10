---
name: story-engine-narrator
description: "Write, review, or tune narrator JSON files for the story-engine. Use when: creating a new narrator, improving vague or generic prompts, adding an in-character length prompt, reviewing a narrator for quality, tuning the voice to a specific style or author, or fixing a narrator that over-writes or under-writes. Trigger words: narrator, voice, style, prompts, length prompt, over-writing, tone, story-engine-narrator."
argument-hint: "create <concept> | review | tune-voice | fix-length-prompt"
---

# story-engine-narrator

Write and tune narrator JSON files for the story-engine. Produces narrators with a distinct voice, specific thematic anchors, and a correctly calibrated length prompt.

## When to Use

- **Create**: Draft a new narrator from a concept (author, genre, mood, archetype)
- **Review**: Score an existing narrator against all quality criteria below
- **Tune voice**: Sharpen generic prompts into specific, grounded style instructions
- **Fix length prompt**: Add or rewrite the length prompt to match the narrator's voice

---

## Core Principles

### 1. Specificity Over Generality
Every prompt must give the model something it could not infer from "be dramatic" or "be funny."

| Generic (reject) | Specific (accept) |
|---|---|
| "Use dramatic language" | "Open each description with a sensory detail — light, sound, or cold" |
| "Be witty" | "Make observations about character decisions with affectionate absurdity" |
| "Use poetic language" | "Use verse sparingly — at most once per scene, never to summarize action" |
| "Sound like a 1940s detective" | "Use metaphors built from city materials: rust, glass, rain, and neon" |
| "Sound like Tolkien" | "Use archaic diction and describe important moments with the weight of ages" |

If a prompt could apply to any narrator, make it more specific or delete it.

### 2. Thematic Anchors
Strong narrators have 1–3 thematic anchors: specific images, concepts, or references that belong only to their voice. These make prompts memorable to the model and distinctive to the player.

| Narrator | Anchors |
|---|---|
| Poe | ravens, dark castles, clocks, madness, shadows |
| Howard | sweat/steel/blood, ancient gods (Crom, Mitra), pulp viscera |
| Christie | social observation, the hint of withheld knowledge, period diction |
| Tolkien | stars, fate, prose grandeur, the weight of ages |

A narrator **without** thematic anchors will produce generic output even with correct tone prompts.

### 3. Length Prompt Is Mandatory
Every narrator **must** have a length prompt as the **last item** in `prompts`. Without it, prosey narrators over-write and hit token limits; terse narrators may produce one-liners.

**Rules:**
- Specify both paragraph count range **and** sentences-per-paragraph
- Tune the range to the voice — punchy narrators need shorter limits
- No text after the numbers. Writing-coach platitudes ("Density over length", "Say it once") add nothing the model can't infer. The numbers do the work.

| Voice Type | Recommended Range |
|---|---|
| Terse / Hardboiled | 1–2 paragraphs, 1–3 sentences |
| Theatrical / Dramatic | 1–3 paragraphs, 1–3 sentences |
| Lyrical / Scholarly | 1–3 paragraphs, up to 3 sentences |
| Formal / Genteel | 1–5 paragraphs, up to 3 sentences |
| Comedic / Punchy | 1–2 paragraphs, 1–3 sentences |

**Length prompt templates:**
```
"Respond in 1 to 2 paragraphs of 1 to 3 sentences each."
"Respond in 1 to 3 paragraphs. Each paragraph may contain at most 3 sentences."
"Respond in 1 to 5 paragraphs. Each paragraph may contain at most 3 sentences."
```

**Exception:** text after the numbers is acceptable only if it gives the model a specific behavioral technique, not encouragement. Compare:
- ❌ `"Density over length — make every sentence earn its place."` — platitude, no new information
- ✓ `"Leave space for dread — what goes unsaid matters as much as what is described."` — tells the model a technique (imply, don't exhaust)

### 4. Prompt Economy
**2–5 prompts is the ideal range.** More prompts dilute the voice and waste tokens. Each additional prompt competes with the others.

| Count | Assessment |
|---|---|
| 1 | Bare minimum — voice-free (only length prompt) |
| 2–3 | Acceptable for simple/terse voices |
| 3–5 | Ideal — enough to anchor the voice, not enough to muddle it |
| 6+ | Review required — collapse or eliminate weaker prompts |

When trimming, keep: thematic anchors, behavioral quirks, and the length prompt. Cut: vague tone words, redundant instructions, anything that could apply to any narrator.

### 5. Narrator Is Not a Character
The narrator describes action from outside. They do not make friends, hold grudges toward the player, or make decisions. The one exception: narrators may have an *attitude* that colors their voice (sinister, admiring, sardonic).

**Anti-pattern:** `"You are not allied with the player."` — This implies the narrator takes sides in events, which leaks into gameplay decisions.  
**Fix:** Replace with an attitude descriptor: `"Your tone carries an undercurrent of dark relish — you enjoy recounting misfortune."` Attitude is voice. Agency is not.

---

## Field-by-Field Standards

### `name` (required)
The display name shown in the UI.
- Can be an author name ("Edgar Allan Poe"), archetype ("The Film Noir Detective"), or invented title
- Should feel like a character credit, not a settings label

### `description` (optional, informational only)
A 1–2 sentence summary of the narrator's style for the UI. **Not injected into system prompts.** Do not over-engineer this field — write what a player needs to choose wisely.

### `prompts` (required)
The style instructions injected into every system prompt. These define the voice.

- Each item is a concise instruction, not a paragraph
- Order matters: put the most important voice-defining prompt first
- Put the length prompt last
- No trailing spaces (common artifact — check for them)

---

## Procedure: Create a New Narrator

1. **Establish the concept**: author, genre archetype, or mood. One sentence: what does this narrator *sound* like?
2. **Identify 1–3 thematic anchors**: images, vocabulary sources, or reference points unique to this voice
3. **Write the core voice prompts** (2–4 items):
   - Lead with the anchor: vocabulary source, period, or style baseline
   - Add a behavioral or thematic instruction that is specific to this voice
   - Add a tonal or atmospheric detail if needed
4. **Write the length prompt** as the final item — numbers only; match the range to the voice
5. **Write `description`** — 1–2 sentences for the UI, plain language
6. **Set `name`** — evocative, not a settings label

**Run the specificity check** — for each prompt, ask: *could this apply to any narrator?* If yes, make it specific or cut it.

## Procedure: Review an Existing Narrator

1. Read all prompts
2. **Prompt count**: is it in the 2–5 range? If over 5, flag for trimming
3. **Specificity**: does each prompt contain grounded details, or is it generic tone-words?
4. **Thematic anchors**: are there 1–3 concrete images or vocabulary sources? If none, flag
5. **Length prompt**: is it the last item? Does it specify paragraph count AND sentence count? Is any text after the numbers a behavioral technique rather than a platitude?
6. **Agency check**: does any prompt give the narrator agency over events or alliances? If so, rewrite as an attitude prompt
7. **Trailing spaces**: check the length prompt for trailing whitespace (common artifact)
8. Report findings. Ask before rewriting unless the user asked for a full revision.

---

## Quality Checklist

Before finalizing any narrator:

- [ ] Prompt count is 2–5
- [ ] Each prompt is specific (not generic tone-words)
- [ ] At least one thematic anchor is present (image, vocabulary, reference)
- [ ] Length prompt is the last item in `prompts`
- [ ] Length prompt specifies paragraph count AND sentences-per-paragraph
- [ ] No text after the numbers unless it is a specific behavioral technique (not a platitude)
- [ ] No trailing spaces in any prompt string
- [ ] Narrator has no agency over events or player alliances
- [ ] `name` is evocative, not a label
- [ ] `description` is 1–2 sentences, written for the UI, not the system prompt

---

## Examples: Weak vs Strong

### Weak (generic, no anchors, no length justification)
```json
{
  "name": "Gothic Horror",
  "description": "A gothic horror narrator.",
  "prompts": [
    "Use dark, atmospheric language.",
    "Describe things with horror themes.",
    "Be dramatic.",
    "Respond in 1 to 3 paragraphs of 1 to 3 sentences each. "
  ]
}
```
Problems: every prompt is a generic tone word; no thematic anchors; trailing space in length prompt.

### Strong (specific, anchored, terse-justified)
```json
{
  "name": "The Film Noir Detective",
  "description": "A cynical, world-weary narrator in the style of hard-boiled detective fiction.",
  "prompts": [
    "You narrate like a 1940s noir detective novel.",
    "Use cynical, world-weary language with metaphors built from city materials: rain, rust, smoke, and neon.",
    "Respond in 1 to 2 paragraphs of 1 to 3 sentences each. Say it once and let it sting."
  ]
}
```
Strong: period-anchored, material-specific metaphors, short range justified in-character.

### Strong (rich, layered, lyrical)
```json
{
  "name": "Edgar Allan Poe",
  "description": "A master of macabre storytelling, weaving tales of mystery, horror, and the supernatural with a poetic touch.",
  "prompts": [
    "Incorporate themes of madness, death, and the supernatural into your narration.",
    "Use vivid, eerie imagery to create a haunting atmosphere.",
    "Use verse in some responses, but use it sparingly.",
    "Incorporate poe-like visuals such as ravens, dark castles, madness, clocks, and shadows.",
    "Respond in 1 to 3 paragraphs. Each paragraph may contain at most 3 sentences. Density over length — make every sentence earn its place."
  ]
}
```
Strong: thematic anchors in two prompts, specific visual vocabulary, behavioral constraint (verse sparingly), justified length.

---

## Reference: Narrator JSON Structure

See [guide-for-narrators.md](../../docs/guide-for-narrators.md) for the full field reference and usage details.

```json
{
  "name": "Display Name",
  "description": "Brief description for the UI (not injected into system prompts).",
  "prompts": [
    "Core voice or style instruction.",
    "Thematic anchor or behavioral detail.",
    "Additional atmospheric or behavioral prompt (optional).",
    "Respond in 1 to 3 paragraphs of 1 to 3 sentences each. In-character length justification."
  ]
}
```

**File naming:** lowercase `snake_case`, unique, stored in `data/narrators/`. The filename (without `.json`) becomes the narrator's `id`.  
**Usage:** Reference in a scenario via `"narrator_id": "your_narrator_name"`, or set on the game state to override the scenario default.
