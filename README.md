# Story Engine
A lightweight narrative engine for immersive, structured text adventures. Game engine inspired by text adventure games of the 70's and 80's, augmented with a modern LLM's conversational capabilities. 

## Architecture
Project includes a Go microservice API and a console app. Console app is lightweight, to demonstrate a barebones gameplay session. 

### Storage Interface

The project uses Redis as the primary storage backend for game state persistence. The storage interface provides:

- **Game State Management**: Create, read, update, and delete operations for game sessions
- **Redis Integration**: High-performance in-memory storage with optional persistence

### LLM Interface

The LLM interface provides an abstraction layer for Large Language Model integration:

- **Current Implementation**: Venice AI service for low-cost and performant responses
- **Interface Design**: Pluggable architecture supporting multiple LLM providers
- **Chat Integration**: Handles conversation context and message formatting
- **Model Management**: API for model initialization and readiness checks

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
POST /gamestate
Content-Type: application/json

# Response: 201 Created
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "chat_history": []
}
```

**Get Game State**
```bash
GET /gamestate/{id}

# Response: 200 OK
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "chat_history": [...]
}
```

**Delete Game State**
```bash
DELETE /gamestate/{id}

# Response: 204 No Content (idempotent)
```

### Scenario Management

**Get Scenario by Filename**
```bash
GET /scenario/pirate.json

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
POST /chat
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

### Config

Create a json config file. Example:

```json
{
  "port": "8080",
  "environment": "dev",
  "log_level": "debug",
  "ollama_url": "http://localhost:11434",
  "venice_api_key": "API_KEY_HERE",
  "model_name": "llama-3.3-70b",
  "redis_url": "localhost:6379"
}
```

*Note: Currently only Venice LLM provider is supported.* 

#### Venice LLM Provider

Venice provides low-cost access to models that are good at storytelling. Recommended models:
- llama-3.3-70b
- dolphin-2.9.2-qwen2-72b

#### Ollama LLM Provider

TODO: Re-enable Ollama support with a proper model. Local performance constraints may block testing. 

### API

````bash
GAME_CONFIG=config.docker.json go run cmd/api/main.go &
````

### Console Client

````bash
GAME_CONFIG=config.docker.json go run cmd/console/main.go
````