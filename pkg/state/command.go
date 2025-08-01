package state

import (
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
)

type CommandType string

const (
	CmdLook      CommandType = "look"
	CmdInventory CommandType = "inventory"
	// CmdMove      CommandType = "move"
	// CmdGet       CommandType = "get"
	// CmdDrop      CommandType = "drop"
	// CmdTalk      CommandType = "talk"
	// CmdAsk       CommandType = "ask"
	// CmdUse       CommandType = "use"
	// CmdHelp      CommandType = "help"
	// CmdQuit CommandType = "quit"
	CmdNone CommandType = "" // No command, used for fallback
)

// parseCommand parses the input string and returns the command type and argument if recognized.
// If not recognized, returns empty string and empty arg.
func parseCommand(input string) (CommandType, string) {
	known := map[string]CommandType{
		"look":      CmdLook,
		"location":  CmdLook,
		"l":         CmdLook,
		"inventory": CmdInventory,
		"i":         CmdInventory,
		// "move":      CmdMove,
		// "m":         CmdMove,
		// "get":       CmdGet,
		// "g":         CmdGet,
		// "drop":      CmdDrop,
		// "d":         CmdDrop,
		// "talk": CmdTalk,
		// "t":    CmdTalk,
		// "ask":  CmdAsk,
		// "use":  CmdUse,
		// "help": CmdHelp,
		// "h":    CmdHelp,
		// "quit": CmdQuit,
		// "q":    CmdQuit,
	}
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return CmdNone, ""
	}
	cmd, ok := known[trimmed]
	if !ok {
		return CmdNone, ""
	}
	return cmd, ""
}

// CommandResult is an early evaluation of a chat prompt.
type CommandResult struct {
	Handled bool   // True if the command was fully resolved and no LLM call is needed
	Message string // Message or prompt to return
	Role    string // Role for the message, e.g. "user", "assistant", "system"
}

// TryHandleCommand looks for shortcut commands and handles them without LLM if possible.
func (gs *GameState) TryHandleCommand(input string) (*CommandResult, error) {
	cmd, _ := parseCommand(input)

	if cmd == "" {
		// Pass the input through if not a recognized command.
		return &CommandResult{
			Handled: false,
			Message: input,
			Role:    chat.ChatRoleUser,
		}, nil
	}

	switch cmd {
	case CmdLook:
		return &CommandResult{
			Handled: true,
			Message: gs.DescribeLocation(),
			Role:    chat.ChatRoleAgent,
		}, nil

	case CmdInventory:
		return &CommandResult{
			Handled: true,
			Message: gs.DescribeInventory(),
			Role:    chat.ChatRoleAgent,
		}, nil

	default:
		return &CommandResult{
			Handled: false,
			Message: input,
			Role:    chat.ChatRoleUser,
		}, nil
	}
}

func (gs *GameState) DescribeLocation() string {
	if loc, ok := gs.WorldLocations[gs.Location]; ok {
		return loc.Description
	}
	return "You are in an unknown location."
}

func (gs *GameState) DescribeInventory() string {
	if len(gs.Inventory) == 0 {
		return "Your inventory is empty."
	}
	return "You have: - " + strings.Join(gs.Inventory, "\n- ")
}
