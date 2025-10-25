# Story Engine
A lightweight narrative engine for immersive, structured text adventures. Game engine inspired by text adventure games of the 70's and 80's, augmented with a modern LLM's conversational capabilities. 

## Features

Features are geared towards a closed-world / on-rails style of D&D adventure. 

- **Scene-Based Narrative** - Linear or branching scenes, used to tell a story over a series of "acts." Confining information to scenes reduces LLM confusion.
- **Location & Map System** - Confines the gameworld to a defined set of locations with movement rules. 
- **Item & Inventory Management** - Player can acquire, drop, give and use items.
- **Player Character System** - Players take the roles of 5e-compatible PC's. PCs are decoupled from scenarios.
- **NPC System** - Story-scoped NPCs with planned mutable properties. Mutable properties aren't well fleshed out or tested yet. 
- **Variables** - Simple vars for tracking story progression. It's relatively easy for an LLM to track these, so they can be combined with conditional logic for powerful game control. 
- **Story Events** - Ability to inject story events into the chat flow.

## Architecture
Project includes a Go microservice API and a console app. 

### Package Organization

The codebase follows a clean architecture with clear separation of concerns:

```
pkg/
├── state/          # Game state data structures (low-level)
├── prompts/        # LLM message construction (high-level)
├── scenario/       # Scenario definitions and rules
├── actor/          # Player characters and NPCs
├── chat/           # Chat message types
└── storage/        # Storage interface

internal/
└── handlers/       # HTTP request handlers and business logic
└── services/       # LLM interface and implementations
└── storage/        # Filesystem and Redis storage implementations
```

### Prompt Builder

The prompt builder package (`pkg/prompts`) provides a fluent interface for constructing LLM chat messages:

- **Separation of Concerns**: Isolates prompt construction logic from game state management
- **Fluent Interface**: Chainable methods for composing complex prompts
- **Automatic Context Assembly**: Combines narrator voice, player character details, scenario rules, game state, and chat history
- **Story Event Injection**: Seamlessly integrates queued story events into the conversation flow
- **Contingency Prompts**: Handles conditional prompts based on game state (variables, turn count, scene)
- **History Windowing**: Manages chat history with configurable limits to control token usage

**Usage Example:**
```go
messages, err := prompts.New().
    WithGameState(gameState).
    WithScenario(scenario).
    WithUserMessage(userInput, "user").
    WithHistoryLimit(20).
    Build()
```

The builder automatically:
- Loads narrator personality and style from embedded game state
- Includes player character details and conditional prompts
- Adds scenario rules and content rating guidelines
- Manages chat history with proper windowing
- Injects story events at the appropriate position
- Appends final reminders or game-end prompts

### Storage Interface

The storage layer uses a **public interface** with **private implementations**:

- **Interface (`pkg/storage/`)**: Defines the storage contract for game state, scenarios, narrators, and PCs
- **Implementation (`internal/storage/`)**: Redis-backed game state persistence and filesystem-backed resource loading
- **Session Isolation**: Each game session identified by unique UUID
- **Embedded Data**: Game states include embedded narrator and player character data for reduced I/O

**Storage Strategy:**
- **Narrator & PC**: Embedded in game state (loaded once at creation, stored in Redis)
- **Scenario**: Referenced by filename (loaded from filesystem per request, enables live updates)
- **Chat History**: Stored in Redis as part of game state
- **Future Optimization**: Scenario caching planned to reduce filesystem I/O

### LLM Interface

The LLM service layer (`internal/services/`) provides an abstraction for Large Language Model integration:

- **Provider Abstraction**: Pluggable architecture supporting multiple LLM providers (Anthropic Claude, VeniceAI)
- **Chat Integration**: Handles conversation context and message formatting
- **Streaming Support**: Real-time response streaming with delta updates
- **Game State Extraction**: Parses LLM responses to extract game state changes (location, inventory, variables)
- **Model Management**: Provider initialization and health checks

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

## API Reference

Complete API documentation is available in the OpenAPI specification:

📖 **[API Documentation](docs/openapi.yaml)** - Full REST API reference with request/response examples

### Quick Overview

The API provides endpoints for:
- **Game State Management** - Create, read, update, and delete game sessions
- **Chat Interaction** - Send messages and receive AI narrator responses (supports streaming)
- **Scenario Management** - Browse and load story scenarios
- **Player Characters** - List and retrieve player character definitions
- **Narrators** - Access narrator personalities and styles
- **Health Check** - Monitor API status and dependencies

All endpoints return JSON responses with consistent error formatting. 

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

## Documentation

- **Scenario Creation**: See [data/scenarios/README.md](data/scenarios/README.md) for complete guide on writing scenarios
- **Player Characters**: See [data/pcs/README.md](data/pcs/README.md) for creating and customizing player characters
- **Narrators**: See [data/narrators/README.md](data/narrators/README.md) for creating custom narrator personalities
- **Console Client**: See [cmd/console/README.md](cmd/console/README.md) for gameplay client documentation