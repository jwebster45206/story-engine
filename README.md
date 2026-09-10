# Story Engine
Closed-world text adventure game: you chat, an LLM narrates, and the engine keeps the world consistent. A Go API accepts each turn immediately and streams the narration; a background worker turns that story into game progress.

## Features

- **Scenes** — linear or branching acts
- **Locations & map** — defined world with movement rules
- **Items & inventory** — acquire, drop, give, use
- **Player characters** — 5e-compatible PCs, decoupled from scenarios
- **NPCs** — story-scoped; mutable properties still early
- **Monsters (v1)** — templated enemy lifecycle
- **Story events** — inject fixed narrative into the chat flow

## Architecture

Story Engine exposes a REST API. Clients create a game session, subscribe to Server-Sent Events (SSE), and send chat turns. Narrative responses are streamed over SSE. 

Redis holds game session state, chat request queue, per-game locks, and a pub/sub channel from worker to API. 

An optional console TUI lives under `cmd/console`.

### Main loop

**Init**
1. `POST /v1/gamestate` — create a session (scenario, optional PC/narrator/provider).
2. `GET /v1/events/gamestate/{id}` — subscribe to SSE before chatting.

**Chat loop**
1. `POST /v1/chat` — enqueue a player message (`202` + `request_id`).
2. Narration arrives on the SSE stream (`request.processing` → `chat.chunk` → `request.completed` / `request.failed`).
3. Structured game state (location, inventory, vars, scenes, …) updates in the background.
4. Engine-driven story events are also queued and streamed over the same SSE channel.

### LLM layer

Each turn calls an LLM twice: a narrator that streams to the player, then a (often cheaper) model that extracts structured game changes. Named providers in config pick the vendor and those two models; the game stores the provider name.

### Authentication

Protected routes take an ES256 JWT (`Authorization: Bearer`). The API validates it and scopes the request to `sub` — a random UUID today, a user id once a standalone auth service exists. Until then, the console and `cmd/token` mint tokens from `auth-key.pem`; the API verifies with the matching public key.

### Binaries

```
cmd/
├── api/            # HTTP API
├── console/        # Optional TUI client
├── token/          # Token generation util
├── validate/       # Util for scenario validation
└── worker/         # Async chat / story-event processor


```

## Running the Service

Copy `config.template.json` to `config.json` (or `config.docker.json` for Compose) and fill in provider keys. 

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
    }
  },
  "redis_url": "localhost:6379"
}
```

Generate an ES256 keypair in the project root (gitignored, and needed for api auth). 

```bash
openssl ecparam -name prime256v1 -genkey -noout -out auth-key.pem
openssl ec -in auth-key.pem -pubout -out auth-key.pub.pem
```

Start the service in docker. 

```bash
# Docker Compose (./data + ./config.docker.json; use redis:6379 in Docker configs)
docker compose up --build -d
DATA_DIR=~/Documents/story-engine-scenarios docker compose up --build -d
docker compose restart story-engine-api story-engine-worker
```

Interact with API via console client.

```bash
# Console client — see cmd/console/README.md
go run ./cmd/console
API_BASE_URL=http://localhost:8080 go run ./cmd/console
```

## Docs

- [API (OpenAPI)](docs/openapi.yaml)
- [Scenarios](docs/guide-for-scenarios.md) · [PCs](docs/guide-for-pcs.md) · [Narrators](docs/guide-for-narrators.md) · [Monsters](docs/guide-for-monsters.md)
- [Scenario validator](cmd/validate/README.md) · [Console](cmd/console/README.md)
