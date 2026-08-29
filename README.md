# Story Engine
A lightweight narrative engine for immersive, structured text adventures. Game engine inspired by text adventure games of the 70's and 80's.

## Features

Features are geared towards a closed-world / on-rails style of D&D adventure. 

- **Scene-Based Narrative** - Linear or branching scenes, used to tell a story over a series of "acts." 
- **Location & Map System** - Attempts to confine the gameworld to a defined set of locations with movement rules. 
- **Item & Inventory Management** - Player can acquire, drop, give and use items.
- **Player Character System** - Players take the roles of 5e-compatible PC's. PCs are decoupled from scenarios.
- **NPC System** - Story-scoped NPCs with planned mutable properties. Mutable properties aren't well fleshed out or tested yet.
- **Monster System (v1)** - Lifecycle-scoped "monsters" for templated enemy creatures.
- **Story Events** - For injecting hardcoded narratives into the chat flow.

## Architecture
Project includes a Go microservice API and a console app. 

### Package Organization

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

- Isolates prompt construction logic from game state management
- Chainable methods for composing complex prompts
- Combines narrator voice, player character details, scenario rules, game state, and chat history
- Handles conditional prompts based on game state (variables, turn count, scene)
- Manages chat history with configurable limits to control token usage

**Usage Example:**
```go
messages, err := prompts.New().
    WithGameState(gameState).
    WithScenario(scenario).
    WithUserMessage(userInput, "user").
    WithHistoryLimit(20).
    Build()
```

### Storage Interface

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

- **Provider Abstraction**: Pluggable architecture supporting multiple LLM providers 
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

Game states are created at session start and maintained throughout the storytelling experience.

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

## Running the Project

### Configuration

Create a JSON configuration file with your service settings:

**Multi-provider config**

Providers are named vendor+model pairings. A vendor is a wire protocol (`anthropic` or `venice`). Adding a provider is JSON-only.

```json
{
  "port": "8080",
  "environment": "dev",
  "log_level": "debug",
  "default_provider": "sonnet",
  "providers": {
    "sonnet": {
      "vendor": "anthropic",
      "display_name": "Claude Sonnet 4.6",
      "api_key": "sk-ant-api03-...",
      "model": "claude-sonnet-4-6",
      "backend_model": "claude-haiku-4-5"
    },
    "venice": {
      "vendor": "venice",
      "display_name": "Venice Uncensored",
      "api_key": "your_venice_api_key_here",
      "model": "venice-uncensored-role-play",
      "backend_model": "qwen3-4b"
    }
  },
  "redis_url": "localhost:6379"
}
```

List providers via `GET /v1/providers`. Pass `provider` on `POST /v1/gamestate` (defaults to `default_provider`).
### API Server

```bash
CONFIG=config.json go run ./cmd/api
```

The worker uses the same env var:

```bash
CONFIG=config.json go run ./cmd/worker
```

### Docker Compose

```bash
# Default: ./data and ./config.docker.json
docker compose up --build -d

# Custom data directory
DATA_DIR=~/Documents/story-engine-scenarios docker compose up --build -d
```

Edit `config.docker.json` on the host, then restart to reload:

```bash
docker compose restart story-engine-api story-engine-worker
```

Use `redis:6379` as `redis_url` in Docker configs (the compose Redis service hostname).

### Console Client

For detailed setup and usage instructions, see the [Console Client README](cmd/console/README.md).

```bash
# Run with default API URL (localhost:8080)
go run cmd/console/*.go

# If using custom API URL
API_BASE_URL=http://localhost:3000 go run cmd/console/*.go
```

## Writing Guides

- **Scenario Creation**: [docs/guide-for-scenarios.md](docs/guide-for-scenarios.md) — complete guide on writing scenarios
- **Player Characters**: [docs/guide-for-pcs.md](docs/guide-for-pcs.md) — creating and customizing player characters
- **Narrators**: [docs/guide-for-narrators.md](docs/guide-for-narrators.md) — creating custom narrator personalities
- **Monsters**: [docs/guide-for-monsters.md](docs/guide-for-monsters.md) — creating monster templates and integrating them into scenarios

### Other Docs
- **API Reference**: [docs/openapi.yaml](docs/openapi.yaml) — full REST API reference
- **Console Client**: [cmd/console/README.md](cmd/console/README.md) — gameplay client documentation