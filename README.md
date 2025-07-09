# Roleplay Agent
API and console app for LLM-powered text roleplay

Gameplay uses elements of 80's text adventure games, augmented with a modern LLM's conversational capabilities. 

```
/roleplay-agent/
├── cmd/
│   ├── api/                # API server (e.g., HTTP server with /chat route)
│   │   └── main.go
│   └── console/            # Console CLI client
│       └── main.go
├── internal/
│   ├── agent/              # Prompt builder, chat logic, model adapter
│   │   └── agent.go
│   ├── config/             # Environment name, Redis address, Model parameters
│   │   └── config.go
│   ├── handlers/           # HTTP handlers
│   │   └── chat.go
│   │   └── handlers.go
│   ├── model/              # LLM interface (Ollama client or mock)
│   │   └── ollama.go
│   │   └── mock.go
│   ├── scenario/
│   │   └── scenario.go     # Loads and manages the template for the roleplay scenario
│   └── state/              # JSON state object + update logic
│       ├── state.go
├── go.mod
└── README.md
└── sample_state.json
└── sample_scenario.json
```

## Architecture
Project will deliver a Go microservice API and a console app to demonstrate a simple gameplay implementation. 

## Initial Responsibilities

`/cmd/api/main.go`

- Starts an HTTP server

`POST /chat`

- Reads input + state, sends to agent, returns response

`/internal/agent/agent.go`

- Builds prompt with system instructions + user input + world state

`/internal/model/ollama.go`

- LLM interface (Ollama client or mock)
- TODO: streaming vs non-streaming generation: prob start with non-streaming only, but study this

`internal/handlers`

- handlers package is responsible for HTTP handlers
- When `POST /chat` is received:
  - Validate the input
  - Send gamestate and request to the agent
  - Return agent's response to user

`internal/agent/`

- Prompt builder, chat logic, model adapter
- agent package communicates with the LLM, and translates its responses to gamestate and user response
- agent package also parses user input for sending to the LLM and storing in gamestate 

`internal/scenario`

- scenario is the template for a roleplay session
- it is loaded from a json file when requested, and cached in redis
- it includes the story that serves as the basis for the session
- it includes clear descriptions of the 2 main characters
- it includes foundational prompt rules for the LLM's character
- it includes end conditions
- TODO: add key-value gauge metrics
- TODO: add inventory system
- TODO: add room system

`/internal/state/`

- Defines game state struct
- Doesn't make a copy of scenario
- Responsible for storing game state in Redis, and retrieving it. 

### Console
`/cmd/console/main.go`

1. Reads user input from stdin
2. Sends to agent logic
3. Prints response
4. Return to #1

## Infrastructure

API runs its own mini LLM locally, and uses it to drive a gameplay session.

docker-compose API structure:
- Main project (go microservice)
- Redis storage
- Ollama LLM