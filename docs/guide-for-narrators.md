# Narrators

Narrators define the voice and style of the game's storytelling. Each narrator is a reusable personality that can be applied to any scenario. Narrator files are stored as JSON in `data/narrators/`.

## File Format

Each narrator is a JSON file with the following structure:

```json
{
  // "id" is generated from the json filename
  "name": "Display Name",
  "description": "Brief description of the narrator's style",
  "prompts": [
    "Voice or style instruction.",
    "Additional style prompts as needed."
  ],
  "rules": [
    "Respond in 1 to 3 paragraphs of 1 to 3 sentences each."
  ]
}
```

**Filename:** File names should be unique, lowercase and snake_case.

### Fields

- **name** (required): Human-readable display name
- **description** (optional): Brief description of what this narrator style is like; informational, for ui, and not used in system prompts
- **prompts** (required): Array of voice and style instructions injected into the system prompt
- **rules** (optional): Array of per-turn constraints injected into the `<rules>` block after every user message; use this for the length rule and any hard behavioral constraints

## Usage

### In Scenarios

Add a `narrator_id` field to your scenario JSON:

```json
{
  "name": "My Scenario",
  "narrator_id": "vincent_price",
  ...
}
```

### In Game Sessions

Players can override the scenario's default narrator when creating a game session by setting the `narrator_id` field in the game state.

## Creating Custom Narrators

1. Create a new JSON file in this directory (e.g., `my_narrator.json`)
2. Follow the file format above
3. Ensure the `id` field matches your filename (without `.json`)
4. Add prompts that define the narrator's personality and style
5. Reference it in scenarios using the `narrator_id` field

### Tips for Writing Narrator Prompts

- **Keep it concise**: 2-5 prompts is ideal. More prompts = more tokens and potentially LLM confusion
- Be specific about tone, writing style, and the narrator's personality
- Keep each prompt short and actionable

### Output Length and Structure

Every narrator **must** include a length rule in the `rules` field. The system prompt provides a soft default ("1 to 3 short paragraphs of 1 to 3 sentences each"), but narrators are responsible for overriding or reinforcing that default to match their voice.

Guidelines:
- **Always include a length rule** in the `rules` array
- Specify both paragraph count and sentences-per-paragraph
- Match the constraint to the narrator's voice — punchy narrators should be shorter, lyrical ones can use the full range
- Without this rule, prosey narrators tend to over-write and hit the token limit

Example length rules:
- `"Respond in 1 to 3 paragraphs. Each paragraph may contain at most 3 sentences."` (Poe, Tolkien, Howard)
- `"Respond in 1 to 2 paragraphs of 1 to 3 sentences each."` (Noir)

## Examples

### Minimal Narrator
```json
{
  "id": "simple",
  "name": "Simple Narrator",
  "description": "Basic, no-frills storytelling",
  "prompts": [
    "Use clear, simple language.",
    "Focus on facts and actions."
  ],
  "rules": [
    "Respond in 1 to 3 paragraphs of 1 to 3 sentences each."
  ]
}
```

### Character-Driven Narrator
```json
{
  "id": "shakespeare",
  "name": "Shakespearean",
  "description": "Dramatic, theatrical narrator in Elizabethan style",
  "prompts": [
    "Speak in the style of William Shakespeare.",
    "Use dramatic language with poetic flourishes.",
    "Occasionally include 'thee', 'thou', and archaic expressions.",
    "Make references to fate, fortune, and the stars."
  ],
  "rules": [
    "Respond in 1 to 3 paragraphs of 1 to 3 sentences each."
  ]
}
```
