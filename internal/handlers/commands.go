package handlers

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

type commandType string

const (
	cmdLook      commandType = "look"
	cmdInventory commandType = "inventory"
	cmdNone      commandType = "" // No command, used for fallback
)

// CommandResult represents the result of attempting to handle a user command.
type CommandResult struct {
	Handled bool   // True if the command was fully resolved and no LLM call is needed
	Message string // Message or prompt to return
	Role    string // Role for the message, e.g. "user", "assistant", "system"
}

// parseCommand parses the input string and returns the command type if recognized.
// If not recognized, returns cmdNone.
func parseCommand(input string) commandType {
	known := map[string]commandType{
		"look":      cmdLook,
		"location":  cmdLook,
		"l":         cmdLook,
		"inventory": cmdInventory,
		"i":         cmdInventory,
	}
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return cmdNone
	}
	if cmd, ok := known[trimmed]; ok {
		return cmd
	}
	return cmdNone
}

// TryHandleCommand attempts to handle shortcut commands without requiring LLM processing.
// Returns a CommandResult indicating whether the command was handled and what message to return.
func TryHandleCommand(gs *state.GameState, input string) *CommandResult {
	cmd := parseCommand(input)

	if cmd == cmdNone {
		// Pass the input through if not a recognized command.
		return &CommandResult{
			Handled: false,
			Message: input,
			Role:    chat.ChatRoleUser,
		}
	}

	switch cmd {
	case cmdLook:
		return &CommandResult{
			Handled: true,
			Message: describeLocation(gs),
			Role:    chat.ChatRoleAgent,
		}

	case cmdInventory:
		return &CommandResult{
			Handled: true,
			Message: describeInventory(gs),
			Role:    chat.ChatRoleAgent,
		}

	default:
		return &CommandResult{
			Handled: false,
			Message: input,
			Role:    chat.ChatRoleUser,
		}
	}
}

// describeLocation returns a description of the player's current location.
func describeLocation(gs *state.GameState) string {
	if loc, ok := gs.WorldLocations[gs.Location]; ok {
		return fmt.Sprintf("%s: %s", loc.Name, loc.Description)
	}
	return "You are in an unknown location."
}

// describeInventory returns a description of the player's inventory.
func describeInventory(gs *state.GameState) string {
	if len(gs.Inventory) == 0 {
		return "Your inventory is empty."
	}
	return "You have:\n- " + strings.Join(gs.Inventory, "\n- ")
}
