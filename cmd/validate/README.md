# Scenario Validator

A command-line utility for validating Story Engine scenario JSON files.

## Installation

From the project root directory:

```bash
go build -o validate cmd/validate/main.go
```

Or run directly without building:

```bash
go run cmd/validate/main.go <scenario.json>
```

## Usage

```bash
./validate <scenario.json>
```

### Examples

```bash
# Validate a single scenario file
./validate data/scenarios/pirate.json

# Run directly with go
go run cmd/validate/main.go data/scenarios/space_disaster.json
```

## What It Validates

### JSON Structure
- **Valid JSON syntax** - Ensures the file contains valid JSON
- **No unknown fields** - Catches typos and unsupported fields (e.g., `story_events` at scenario level)
- **Proper unmarshaling** - Validates against the Go struct definitions

### ID Format Validation
All IDs must be lowercase snake_case:
- Scene IDs (keys in `scenes` map)
- Location IDs (keys in `locations` maps)
- NPC IDs (keys in `npcs` maps)
- Referenced IDs in conditionals

### Conditional Structure
- **Non-empty conditions** - Ensures `when` clauses have at least one condition
- **Non-empty actions** - Ensures `then` clauses have at least one action (scene_change, game_ended, or prompt)
- **Variable names** - Validates that variable names in `vars` are lowercase snake_case
- **Location references** - Checks that location references use proper ID format
- **Scene references** - Validates that scene_change.to references use proper ID format

## Exit Codes

- **0** - Validation successful
- **1** - Validation failed (with detailed error messages)

## Common Issues

### ID Format Errors
```
- location ID 'Castle Gates' should be lowercase snake_case
```
**Fix:** Use `castle_gates` instead of `Castle Gates`

### Unknown Fields
```
json: unknown field "story_events"
```
**Fix:** Move `story_events` from scenario level to scene level

### Empty Conditionals
```
- conditional in scene main has empty 'when' clause - no conditions specified
```
**Fix:** Add at least one condition (vars, turn counters, location, etc.)