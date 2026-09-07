# Console Client

A terminal UI for the Story Engine, built with [Charm Bracelet's](https://charm.sh/) Bubble Tea framework.

## Features

- **Chat**: Send messages and receive streamed narrator responses
- **Game State**: Inventory, location, scene, and turn in the right pane
- **Responsive Layout**: Adjusts to terminal size
- **Keyboard Navigation**: Full keyboard support

## Setup

### Prerequisites

- Go 1.27.1 or later
- Running Story Engine API server

### Configuration

The console needs `auth-key.pem` in the working directory (see the [root README](../../README.md) for key generation). The process exits if the private key is missing.

| Env | Default |
|-----|---------|
| `API_BASE_URL` | `http://localhost:8080` |

Each request is sent with `Authorization: Bearer` and an ES256 JWT.

### Running the Client

```bash
go run ./cmd/console

API_BASE_URL=http://your-api-server:8080 go run ./cmd/console
```

## How It Works

### Startup Flow

1. Select scenario, character, play style, and provider (provider picker is skipped when only one is configured)
2. A game state is created via the API
3. The main interface loads with the opening narrative

### User Interface

Split-pane layout:

**Left Panel (Chat)**:
- Story narrative and conversation history
- Message input field at the bottom
- Automatic text wrapping and formatting

**Right Panel (Game State)**:
- Details about game state (inventory, location, etc.)
- Intended for both gameplay and debugging

### Message Flow

1. User types a message and presses Enter
2. The client posts to `/v1/chat` and listens on the game's SSE stream
3. Chunks render as they arrive; game state refreshes when the request completes

### Keyboard Shortcuts

- **Ctrl+C** or **Esc**: Confirm quit
- **Ctrl+N**: Confirm new game (returns to scenario selection)
- **Ctrl+E**: Export chat history to markdown
- **Ctrl+S**: Save game state JSON
- **Ctrl+R**: Refresh game state from the server
- **Ctrl+Y**: Copy game state ID to clipboard
- **Ctrl+Z**: Clear the text input field
- **Enter**: Send message
- **Arrow Keys**: Scroll through chat history
- **PgUp/PgDown**: Scroll chat viewport by page
- **Home/End**: Jump to top/bottom of chat
