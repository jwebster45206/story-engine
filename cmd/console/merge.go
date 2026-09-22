package main

import (
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// mergeServerGameState reconciles the authoritative server game state with any locally
// pending messages that have been optimistically appended but not yet observed from
// the server. A stale snapshot whose chat is a prefix of local does not clobber the
// suffix (in-flight strike-back). Pending users are matched by occurrence count.
func (m *ConsoleUI) mergeServerGameState(serverGS *state.GameState) {
	if serverGS == nil {
		return
	}
	origLen := 0
	var localHist []chat.ChatMessage
	if m.gameState != nil {
		origLen = len(m.gameState.ChatHistory)
		localHist = m.gameState.ChatHistory
	}

	keepLocal := false
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

		keepLocal = chatHistoryPrefix(serverGS.ChatHistory, localHist)
		if !keepLocal {
			m.gameState.ChatHistory = make([]chat.ChatMessage, len(serverGS.ChatHistory))
			copy(m.gameState.ChatHistory, serverGS.ChatHistory)
		}

		if serverGS.IsEnded {
			m.processing = false
		}
	}

	serverCounts := countChatMessages(serverGS.ChatHistory)
	histCounts := countChatMessages(m.gameState.ChatHistory)
	seenPending := map[string]int{}
	var stillPending []chat.ChatMessage
	for _, pm := range m.pendingUserMessages {
		k := chatMessageKey(pm)
		seenPending[k]++
		if seenPending[k] <= serverCounts[k] {
			continue
		}
		stillPending = append(stillPending, pm)
		if seenPending[k] > histCounts[k] {
			m.gameState.ChatHistory = append(m.gameState.ChatHistory, pm)
			histCounts[k]++
		}
	}
	m.pendingUserMessages = stillPending

	if len(m.gameState.ChatHistory) != origLen {
		m.isStreaming = false
		m.streamingMessageIdx = -1
		m.streamingContent = ""
		m.writeChatContent()
	}
}

func chatMessageKey(m chat.ChatMessage) string {
	return m.Role + "\x00" + m.Content
}

func countChatMessages(hist []chat.ChatMessage) map[string]int {
	n := make(map[string]int, len(hist))
	for _, m := range hist {
		n[chatMessageKey(m)]++
	}
	return n
}

func chatHistoryPrefix(prefix, full []chat.ChatMessage) bool {
	if len(prefix) > len(full) {
		return false
	}
	for i := range prefix {
		if prefix[i].Role != full[i].Role || prefix[i].Content != full[i].Content {
			return false
		}
	}
	return true
}
