---
name: story-engine-pc
description: "Write, review, or tune Player Character (PC) JSON files for the story-engine. Use when: creating a new PC, improving a vague description, adding physical description, writing psychological profile, fixing story bleed in background or contingency_prompts, tightening the description field, or reviewing a PC for quality. Trigger words: PC, player character, background, description, physical description, psychology, story bleed, opening_prompt, contingency_prompts."
argument-hint: "create <concept> | review | physical-description | psychology | fix-story-bleed | tune-prompts"
---

# story-engine-pc

Write and tune Player Character JSON files for the story-engine. Produces PCs with concrete physical descriptions, clear psychological profiles, and zero story bleed.

## When to Use

- **Create**: Draft a new PC from a concept (name, genre, class idea)
- **Review**: Score an existing PC against all quality criteria below
- **Physical description**: Add or improve physical appearance in `background`
- **Psychology**: Add or improve a character's psychological profile in `background`
- **Fix story bleed**: Remove plot-specific content from `background`, `description`, `opening_prompt`, or `contingency_prompts`
- **Tune prompts**: Sharpen `contingency_prompts` so they guide behavior, not plot

---

## Core Principles

### 1. Concrete Over Vague
Every descriptor must be grounded in something specific. Generic labels earn nothing.

| Vague (reject) | Concrete (accept) |
|---|---|
| "tall and imposing" | "six-foot-two and broad across the shoulders" |
| "a renowned hunter" | "a hunter who has killed seventeen vampires" |
| "beautiful" | "sharp cheekbones, dark circles under her eyes, never smiles first" |
| "brave and reckless" | "never retreats unless someone else is in danger" |
| "speaks with authority" | "interrupts people who speak slowly; doesn't apologize" |

If you cannot make a descriptor concrete, delete it.

### 2. Physical Description Is Mandatory
The `background` field must contain a physical introduction paragraph. If it is missing, add one.

**Required elements:**
- Height and build (specific, not relative)
- Hair, eyes, skin — one precise detail each, not all three
- One or two distinguishing features (scar, posture, hands, voice quality)
- Default clothing or gear silhouette — what does the character look like when they walk into a room?

**Avoid:** beauty judgments, comparisons to ideals, symmetry praise. Write what a stranger would notice.

### 3. Psychological Overview Is Mandatory
The `background` field must establish who the character is inside. Include all four of these:

- **Want**: What do they actively pursue? (concrete goal, not "greatness" or "justice")
- **Fear**: What do they avoid or deny? (specific, not "failure" or "death")
- **Defense**: What do they do under pressure before they can help it? (humor, silence, aggression, deflection)
- **Belief**: What do they know to be true that shapes every decision? (a rule they live by)

These don't need headers. Weave them into prose. But all four must be present.

### 4. No Story Bleed
The PC file defines who the character IS. It does not describe what will happen in a scenario.

**Story bleed (remove):**
- References to specific enemies, villains, or named NPCs from a scenario
- Future-tense vows or quests ("she has vowed to…", "he seeks to destroy…")
- Narrator-voice claims about reputation ("renowned", "unmatched", "legendary") without a grounding fact
- Location-specific details that only apply to one scenario
- Plot events that happen *during* a game ("after the ship sank…" unless this is backstory prior to game start)

**Allowed:**
- Backstory events that ended before the story begins
- Relationships from the character's past (not from an active scenario)
- Skills, habits, and worldview that emerged from those events

**Test:** Could this PC be dropped into a different, compatible scenario and still make sense? If not, something needs to be scenario-scoped instead of PC-scoped. Move plot-specific content to the scenario's `scene_prompt` or `contingency_prompts`.

---

## Field-by-Field Standards

### `description` (1–2 sentences)
The fast-read identity of the character. Must answer: who are they, and what is their defining quality?

- Lead with something immediate and visual or behavioral — not their job title
- Include personality in the same breath as context
- Do not use "renowned", "legendary", "known for" as the main descriptor without a concrete hook
- Do not summarize the plot

**Examples:**
```
WEAK: "A renowned vampire hunter and occult scholar who has devoted his life to destroying supernatural evil."
BETTER: "A methodical hunter who catalogs every kill in a leather journal and treats each new vampire as a problem not yet solved."

WEAK: "A young apprentice mage eager to prove herself."
BETTER: "A mage apprentice whose spells work about seventy percent of the time and who considers this an acceptable risk."
```

### `background` (3–5 paragraphs, suggested structure)
1. **Physical introduction** — What does this person look like and how do they occupy a room?
2. **Formative history** — What shaped them? Specific events, not montages.
3. **Psychological profile** — Want, fear, defense, belief woven into prose.
4. **Present state** — Where are they mentally and materially as the story begins? No plot events, just readiness.

### `contingency_prompts`
These are behavioral guidelines for the narrator. They are NOT plot cues.

**Each prompt must:**
- Describe a behavior, speech pattern, or emotional response
- Be specific enough that two different narrators would produce similar output
- Avoid naming scenario-specific people, places, or events

**Examples:**
```
WEAK: "Alexandra is suspicious of Professor Blackwood." (story bleed—Blackwood is a scenario NPC)
BETTER: "Alexandra becomes visibly colder toward authority figures who speak in abstractions or refuse direct questions."

WEAK: "Owen becomes protective when Evie is in danger." (story bleed—Evie is a scenario NPC)
BETTER: "When someone Owen has decided to trust is threatened, he stops joking and becomes precise and quiet."

WEAK: "Van Helsing is methodical in combat." (too vague)
BETTER: "Van Helsing narrates his own tactics aloud when fighting—not from arrogance, but because speaking the plan keeps his hands steady."
```

**Simple strings** (always active): use for speech patterns, baseline posture, non-negotiable quirks.  
**Conditional objects**: use for state changes — injury, emotional escalation, resource depletion.

### `opening_prompt` (optional)
If present, this speaks in the character's voice or sets their frame entering the story. It should NOT:
- Reference the scenario's villain by name
- Summarize the scenario's plot
- Make claims about the character's skill in narrator-voice ("Your expertise is unmatched")

If `opening_prompt` contains story bleed, move the factual content to `background` and rewrite the prompt as a character voice or internal thought.

---

## Procedure: Create a New PC

1. **Establish core concept**: name, genre/setting, class/role, one-line dramatic tension ("a soldier who doesn't know the war is over")
2. **Draft stats** using the guide's standard arrays — match numbers to the character concept
3. **Write `description`** — 1–2 sentences, concrete, no plot
4. **Write `background`** in four paragraphs: physical, history, psychology, present state
5. **Write `contingency_prompts`** — minimum 3, covering baseline voice, a pressure response, and one conditional state change
6. **Assemble `inventory`** — items that reflect history and utility, not fantasy wish-lists
7. **Run story-bleed check** — scan every text field for scenario-specific nouns (named NPCs, place names, event references) and either remove or generalize them
8. **Run vagueness check** — flag any adjective or claim that could apply to 50% of fictional characters; replace or delete

## Procedure: Review an Existing PC

1. Read every text field
2. Check `description`: concrete? no plot? ≤2 sentences?
3. Check `background` for: physical paragraph (present?), psychological four-pack (want/fear/defense/belief all present?), story bleed
4. Check each `contingency_prompt`: behavior or plot? specific enough to bind narrator output?
5. Check `opening_prompt` if present: character voice or narrator-voice? story bleed?
6. Check stats: do the numbers support the written character? Combat stats consistent with class/background?
7. Report findings. Ask before rewriting unless the user asked for full revision.

---

## Reference: PC JSON Structure

See [guide-for-pcs.md](../../docs/guide-for-pcs.md) for the full field reference, stat guidelines, and conditional prompt syntax.

Key stat ranges (D&D 5e):
- 8: notable weakness
- 10–11: average
- 13–14: trained/talented
- 16–17: exceptional
- 19+: legendary

`contingency_prompt` condition fields (AND logic): `vars`, `min_turns`, `turn_counter`, `min_scene_turns`, `scene_turn_counter`, `location`
