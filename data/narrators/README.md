# Narrators

Narrators define the voice and style of the game's storytelling. Each narrator is a reusable personality that can be applied to any scenario.

## File Format

Each narrator is a JSON file with the following structure:

```json
{
  // "id" is generated from the json filename
  "name": "Display Name",
  "description": "Brief description of the narrator's style",
  "prompts": [
    "Instruction 1 for the narrator's voice or style.",
    "Instruction 2 for the narrator's behavior.",
    "Additional prompts as needed."
  ]
}
```

**Filename:** File names should be unique, lowercase and snake_case. Narrators should be stored in data/narrators directory. 

### Fields

- **name** (required): Human-readable display name
- **description** (optional): Brief description of what this narrator style is like; informational, for ui, and not used in system prompts
- **prompts** (required): Array of instructions that shape the narrator's voice and style

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
  ]
}
```
