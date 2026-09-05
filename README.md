# Story Engine
A lightweight narrative engine for immersive, structured text adventures. Game engine inspired by text adventure games of the 70's and 80's.

## Features

- **Scenes** — linear or branching acts
- **Locations & map** — defined world with movement rules
- **Items & inventory** — acquire, drop, give, use
- **Player characters** — 5e-compatible PCs, decoupled from scenarios
- **NPCs** — story-scoped; mutable properties still early
- **Monsters (v1)** — templated enemy lifecycle
- **Story events** — inject fixed narrative into the chat flow

## Architecture

The Story Engine exposes a REST API for interactive, closed-world adventures. Clients create a game session, subscribe to Server-Sent Events, and send chat turns that a background worker processes with an LLM. Redis holds session state, the request queue, per-game locks, and SSE pub/sub between the API (`cmd/api`) and worker (`cmd/worker`). An optional console TUI lives under `cmd/console`.

### Main loop

**Init**
1. `POST /v1/gamestate` — create a session (scenario, optional PC/narrator/provider).
2. `GET /v1/events/gamestate/{id}` — subscribe to SSE before chatting.

**Chat loop**
1. `POST /v1/chat` — enqueue a player message (`202` + `request_id`).
2. Narration arrives on the SSE stream (`request.processing` → `chat.chunk` → `request.completed` / `request.failed`).
3. Structured game state (location, inventory, vars, scenes, …) updates in the background: `DeltaUpdate` extracts a delta, then `state.Applier` applies it.
4. Engine-driven story events are also queued and streamed over the same SSE channel.

### LLM layer

Providers are named **vendor + model** entries in config. A **vendor** is a wire protocol implemented in Go; adding a provider is JSON-only. `internal/llm.Registry` maps provider names to `LLMService`:

- `ChatStream` — narrator responses (`model`)
- `DeltaUpdate` — structured state extraction, often via a cheaper `backend_model`

`pkg/prompts` builds the message lists; the worker calls the registry by the game’s `provider`. List configured providers with `GET /v1/providers`.

### Layout

```
cmd/
├── api/            # HTTP API
├── worker/         # Async chat / story-event processor
├── validate/       # Scenario validation CLI
└── console/        # Optional TUI client

pkg/
├── state/          # Game state + Applier (delta application)
├── prompts/        # LLM message construction
├── scenario/       # Scenario definitions and rules
├── actor/          # PCs, NPCs, monsters
├── chat/           # Chat message types
├── queue/          # Queue request models
└── storage/        # Storage interface

internal/
├── handlers/       # HTTP handlers
├── worker/         # Queue consumer + chat processor
├── llm/            # LLM providers and registry
├── queue/          # Redis work queue
├── events/         # Redis pub/sub for SSE
└── storage/        # Redis + filesystem implementations
```

API: [docs/openapi.yaml](docs/openapi.yaml) — gamestate, chat, events, content browsers, providers, health.

## Running

**Config** — JSON; providers are named vendor+model pairs:

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


```bash
# API + worker (same CONFIG)
CONFIG=config.json go run ./cmd/api
CONFIG=config.json go run ./cmd/worker

# Docker Compose (./data + ./config.docker.json; use redis:6379 in Docker configs)
docker compose up --build -d
DATA_DIR=~/Documents/story-engine-scenarios docker compose up --build -d
docker compose restart story-engine-api story-engine-worker

# Console client — see cmd/console/README.md
go run cmd/console/*.go
API_BASE_URL=http://localhost:3000 go run cmd/console/*.go
```

## Docs

- [API (OpenAPI)](docs/openapi.yaml)
- [Scenarios](docs/guide-for-scenarios.md) · [PCs](docs/guide-for-pcs.md) · [Narrators](docs/guide-for-narrators.md) · [Monsters](docs/guide-for-monsters.md)
- [Scenario validator](cmd/validate/README.md) · [Console](cmd/console/README.md)
