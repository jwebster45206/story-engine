# Console Client

A terminal-based user interface for the Story Engine, built with [Charm Bracelet's](https://charm.sh/) Bubble Tea framework. The console client provides an immersive text adventure experience directly in your terminal.

## Features

- **Real-time Chat**: Send messages and receive AI-generated responses
- **Game State Display**: View current game information, variables, and session details
- **Responsive Layout**: Automatically adjusts to terminal size
- **Keyboard Navigation**: Full keyboard support with intuitive controls

## Setup

### Prerequisites

- Go 1.27.1 or later
- Running Story Engine API server

### Configuration

The console needs the API base URL and an ES256 **private** key matching the engine’s `jwt_public_key`. Flags win over env. The process exits if no private key is set.

| Flag | Env | Default |
|------|-----|---------|
| `--jwt-private-key-file` | `STORY_ENGINE_JWT_PRIVATE_KEY_FILE` | required unless `STORY_ENGINE_JWT_PRIVATE_KEY` is set |
| | `STORY_ENGINE_JWT_PRIVATE_KEY` | PEM contents (alternative to a file) |
| `--principal` | `STORY_ENGINE_PRINCIPAL` | generated UUID (printed to stderr; not persisted) |
| | `API_BASE_URL` | `http://localhost:8080` |

Each request is sent with `Authorization: Bearer` and a freshly minted ES256 JWT (`sub` = principal, `exp` ~1h). This local minting is a stand-in for a token from an auth service.

Generate a keypair (same commands as the [root README](../../README.md)):

```bash
openssl ecparam -name prime256v1 -genkey -noout -out jwt-ec.pem
openssl ec -in jwt-ec.pem -pubout -out jwt-ec.pub.pem
```

Put the public PEM in the engine config. Pass the private key to the console.

### Running the Client

```bash
go run ./cmd/console --jwt-private-key-file=jwt-ec.pem

API_BASE_URL=http://your-api-server:8080 go run ./cmd/console \
  --jwt-private-key-file=jwt-ec.pem \
  --principal=22222222-2222-4222-8222-222222222222
```

## How It Works

### Startup Flow

1. **Scenario Selection**: On startup, the client displays a modal with available scenarios
2. **Game Creation**: After selecting a scenario, a new game state is created via the API
3. **Chat Interface**: The main interface loads with the scenario's opening narrative

### User Interface

The console client uses a split-pane layout:

**Left Panel (Chat)**:
- Story narrative and conversation history
- Message input field at the bottom
- Automatic text wrapping and formatting

**Right Panel (Game State)**:
- Details about game state (inventory, location, etc.)
- Intended for both gameplay and debugging

### Message Flow

1. User types message and presses Enter
2. Message is sent to the Story Engine API
3. AI processes the message within the scenario context
4. Response is formatted and displayed in the chat panel
5. Game state is automatically refreshed

### Keyboard Shortcuts

- **Ctrl+C** or **Esc**: Quit the application
- **Ctrl+N**: Start a new game (resets to scenario selection)
- **Ctrl+E**: Export chat history to markdown file
- **Ctrl+Y**: Copy game state ID to clipboard
- **Ctrl+Z**: Clear the text input field
- **Enter**: Send message
- **Arrow Keys**: Scroll through chat history
- **PgUp/PgDown**: Scroll chat viewport by page
- **Home/End**: Jump to top/bottom of chat