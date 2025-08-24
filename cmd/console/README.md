# Console Client

A terminal-based user interface for the Story Engine, built with [Charm Bracelet's](https://charm.sh/) Bubble Tea framework. The console client provides an immersive text adventure experience directly in your terminal.

## Features

- **Real-time Chat**: Send messages and receive AI-generated responses
- **Game State Display**: View current game information, variables, and session details
- **Responsive Layout**: Automatically adjusts to terminal size
- **Keyboard Navigation**: Full keyboard support with intuitive controls

## Setup

### Prerequisites

- Go 1.21 or later
- Running Story Engine API server

### Configuration

The console client only needs to know where the API server is running. By default, it connects to `http://localhost:8080`.

To use a different API server address:

```bash
export API_BASE_URL=http://your-api-server:8080
```

### Running the Client

```bash
# Run with default API URL (localhost:8080)
go run cmd/console/*.go

# Run with custom API URL
API_BASE_URL=http://your-api-server:8080 go run cmd/console/*.go
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
- Metadata about game state

### Message Flow

1. User types message and presses Enter
2. Message is sent to the Story Engine API
3. AI processes the message within the scenario context
4. Response is formatted and displayed in the chat panel
5. Game state is automatically refreshed