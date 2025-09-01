# Story Engine
A lightweight narrative engine for immersive, structured text adventures. Game engine inspired by text adventure games of the 70's and 80's, augmented with a modern LLM's conversational capabilities. 

## Features

The Story Engine supports a comprehensive set of game mechanics for creating rich text adventures:

### Scene-Based Narrative
- **Linear Progression**: Stories can progress through defined scenes with specific contexts and characters
- **Branching Support**: Framework supports branching narratives (linear implementation currently, but designed for branching)
- **Scene Transitions**: Automatic scene changes based on game conditions and player actions

### Location & Map System  
- **Room-Based Navigation**: Players move between defined locations with explicit exit connections
- **Exit Restrictions**: Movement limited to available exits from current location - no teleportation
- **Location Descriptions**: Rich descriptions and contextual details for each area

### Item & Inventory Management
- **Player Inventory**: Track items carried by the player
- **Location Items**: Items available in specific locations
- **Item Interactions**: Pick up, use, and give items with proper state tracking
- **Transaction Control**: Prevents item duplication and ensures realistic acquisition mechanics

### NPC System
- **Character Presence**: NPCs with specific locations, descriptions, and personalities  
- **Dynamic Behavior**: NPCs can move between locations and change demeanor based on story events
- **Dialogue Integration**: Named character speech with proper formatting
- **Inventory Support**: NPCs can carry and exchange items

### Variables & Game State
- **Custom Variables**: Track story-specific flags and counters (implementation varies by LLM model)
- **Persistent State**: Variables maintained throughout the game session
- **Conditional Logic**: Story branching based on variable values

### Turn Tracking
- **Session Counters**: Track total number of interactions across the entire game
- **Scene Counters**: Track interactions within individual scenes
- **Reliability Note**: More consistent with some LLM models than others

### Game End Conditions
- **Narrative Endings**: Stories can conclude based on player actions and story progression
- **Conditional Endings**: Multiple possible endings based on game state
- **Final Response**: Special handling for concluding game sessions with proper closure

### Not Implemented
- **Combat**: Potential roadmap item

## Architecture
Project includes a Go microservice API and a console app. Console app is lightweight, to demonstrate a barebones gameplay session. 

### Storage Interface

The project uses Redis as the primary storage backend for game state persistence. The storage interface provides:

- **Game State Management**: Create, read, update, and delete operations for game sessions
- **Redis Integration**: High-performance in-memory storage with optional persistence

### LLM Interface

The LLM interface provides an abstraction layer for Large Language Model integration:

- **Interface Design**: Pluggable architecture supporting multiple LLM providers
- **Chat Integration**: Handles conversation context and message formatting
- **Model Management**: API for model initialization and readiness checks
- **Provider Implementations**: Anthropic Claude and VeniceAI

### Scenario and Rules

Scenarios define the template and rules for storytelling sessions:

- **Narrative Foundation**: Each scenario provides the story context and setting for gameplay
- **Character Definitions**: Clear descriptions of main characters and NPCs
- **LLM Prompt Rules**: Foundational guidelines that shape the AI's storytelling behavior
- **Conversation Formatting**: Rules for character dialogue presentation (double line breaks, character names with colons)
- **Game Boundaries**: Guidelines for staying in character and handling player actions

### GameState

GameState is a storytelling session, including conversation history and session metadata. Each game state is uniquely identified by a UUID and contains:

- **Session ID**: Unique identifier for tracking individual gameplay sessions
- **Chat History**: Complete conversation log between user and AI agent
- **Serialization**: JSON-based storage format for persistence and retrieval

Game states are created at session start and maintained throughout the storytelling experience. Future enhancements may include location tracking, inventory systems, and game flags.

## Endpoints

The API provides RESTful endpoints for game management and chat interaction:

### Health Check
```bash
GET /health
```
Returns API status and health information.

### Game State Management

**Create Game State**
```bash
POST /v1/gamestate
Content-Type: application/json

# Response: 201 Created
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "chat_history": []
}
```

**Get Game State**
```bash
GET /v1/gamestate/{id}

# Response: 200 OK
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "chat_history": [...]
}
```

**Delete Game State**
```bash
DELETE /v1/gamestate/{id}

# Response: 204 No Content (idempotent)
```

### Scenario Management

**Get Scenario by Filename**
```bash
GET /v1/scenarios/pirate.json

# Response: 200 OK
{
  "title": "Pirate Adventure",
  "description": "A swashbuckling adventure on the high seas...",
  "characters": [...],
  "rules": [...]
}
```

### Chat Interaction

**Send Chat Message**
```bash
POST /v1/chat
Content-Type: application/json

{
  "game_state_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "I examine the treasure chest."
}

# Response: 200 OK
{
  "message": "The ancient chest creaks as you approach. Its brass hinges are green with age, and strange symbols are carved into the weathered wood.\n\nDavey: \"Careful there, Captain. That chest has been waiting here longer than any of us have been alive.\""
}
```

### Error Responses
All endpoints return consistent error format:
```json
{
  "error": "Descriptive error message"
}
``` 

## Running the Project

### Configuration

Create a JSON configuration file with your service settings:

**Anthropic Claude**
```json
{
  "port": "8080",
  "environment": "dev",
  "log_level": "debug",
  "llm_provider": "anthropic",
  "anthropic_api_key": "sk-ant-api03-...",
  "model_name": "claude-sonnet-4-20250514",
  "redis_url": "localhost:6379"
}
```

**Venice AI**
```json
{
  "port": "8080",
  "environment": "dev", 
  "log_level": "debug",
  "llm_provider": "venice",
  "venice_api_key": "your_venice_api_key_here",
  "model_name": "llama-3.3-70b",
  "redis_url": "localhost:6379"
}
```

### API Server

```bash
GAME_CONFIG=config.json go run cmd/api/main.go
```

### Console Client

For detailed setup and usage instructions, see the [Console Client README](cmd/console/README.md).

```bash
# Run with default API URL (localhost:8080)
go run cmd/console/*.go

# If using custom API URL
API_BASE_URL=http://localhost:3000 go run cmd/console/*.go
```