package main

import (
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// mergeServerGameState reconciles the authoritative server game state with any locally
// pending messages (both user and assistant) that have been optimistically appended
// but not yet observed from the server. Matching heuristic: role+content.
// If chat history changes, rewrites chat viewport content while preserving scroll pin.
func (m *ConsoleUI) mergeServerGameState(serverGS *state.GameState) {
	if serverGS == nil {
		return
	}
	origLen := 0
	if m.gameState != nil {
		origLen = len(m.gameState.ChatHistory)
	}
	if m.gameState == nil {
		m.gameState = serverGS
	} else {
		m.gameState.ID = serverGS.ID
		m.gameState.ModelName = serverGS.ModelName
		m.gameState.Scenario = serverGS.Scenario
		m.gameState.SceneName = serverGS.SceneName
		m.gameState.NPCs = serverGS.NPCs
		m.gameState.WorldLocations = serverGS.WorldLocations
		m.gameState.Location = serverGS.Location
		m.gameState.Inventory = serverGS.Inventory
		m.gameState.TurnCounter = serverGS.TurnCounter
		m.gameState.SceneTurnCounter = serverGS.SceneTurnCounter
		m.gameState.Vars = serverGS.Vars
		m.gameState.IsEnded = serverGS.IsEnded
		m.gameState.ContingencyPrompts = serverGS.ContingencyPrompts
		m.gameState.ChatHistory = make([]chat.ChatMessage, len(serverGS.ChatHistory))
		copy(m.gameState.ChatHistory, serverGS.ChatHistory)

		// Stop polling if game has ended
		if serverGS.IsEnded {
			m.pollingActive = false
		}
	}

	if len(m.pendingUserMessages) > 0 {
		for _, pm := range m.pendingUserMessages {
			found := false
			for _, existing := range m.gameState.ChatHistory {
				if existing.Role == pm.Role && existing.Content == pm.Content {
					found = true
					break
				}
			}
			if !found {
				// Append pending messages at the end to maintain proper order
				m.gameState.ChatHistory = append(m.gameState.ChatHistory, pm)
			}
		}
		// Filter out any pending messages that have now appeared in server data
		var stillPending []chat.ChatMessage
		for _, pm := range m.pendingUserMessages {
			present := false
			for _, existing := range m.gameState.ChatHistory {
				if existing.Role == pm.Role && existing.Content == pm.Content {
					present = true
					break
				}
			}
			if !present { // Should rarely happen; keep if somehow not present
				stillPending = append(stillPending, pm)
			}
		}
		m.pendingUserMessages = stillPending
	}

	// If chat history length changed or pending merges occurred, re-render chat (but don't lose scroll pin state)
	if len(m.gameState.ChatHistory) != origLen {
		// Reset streaming state since chat history has changed
		// This ensures external messages can start streaming fresh
		m.isStreaming = false
		m.streamingMessageIdx = -1
		m.streamingContent = ""
		m.writeChatContent()
	}
}
