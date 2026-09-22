package main

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func newTestUI() ConsoleUI {
	return NewConsoleUI(&ConsoleConfig{}, nil)
}

func TestMergeServerGameState(t *testing.T) {
	assistant := chat.ChatMessage{Role: "assistant", Content: "You are on deck."}
	user := chat.ChatMessage{Role: "user", Content: "look around"}
	followUp := chat.ChatMessage{Role: "assistant", Content: "Waves crash."}
	strike := chat.ChatMessage{Role: "user", Content: "Giant Rat strikes Felix."}

	tests := []struct {
		name   string
		setup  func(m *ConsoleUI)
		server *state.GameState
		check  func(t *testing.T, m ConsoleUI)
	}{
		{
			name: "nil server leaves local state",
			setup: func(m *ConsoleUI) {
				m.gameState = &state.GameState{SceneName: "dock"}
			},
			server: nil,
			check: func(t *testing.T, m ConsoleUI) {
				if m.gameState == nil || m.gameState.SceneName != "dock" {
					t.Fatalf("local game state changed: %+v", m.gameState)
				}
			},
		},
		{
			name:  "empty local state takes server pointer",
			setup: func(m *ConsoleUI) {},
			server: &state.GameState{
				SceneName:   "hold",
				ChatHistory: []chat.ChatMessage{{Role: "assistant", Content: "Welcome."}},
			},
			check: func(t *testing.T, m ConsoleUI) {
				if m.gameState == nil || m.gameState.SceneName != "hold" {
					t.Fatalf("did not assign server state: %+v", m.gameState)
				}
				if got := len(m.gameState.ChatHistory); got != 1 {
					t.Fatalf("ChatHistory len = %d, want 1", got)
				}
			},
		},
		{
			name: "pending user not on server stays pending after prefix keep",
			setup: func(m *ConsoleUI) {
				m.gameState = &state.GameState{ChatHistory: []chat.ChatMessage{assistant, user}}
				m.pendingUserMessages = []chat.ChatMessage{user}
			},
			server: &state.GameState{
				SceneName:   "deck",
				ChatHistory: []chat.ChatMessage{assistant},
			},
			check: func(t *testing.T, m ConsoleUI) {
				if m.gameState.SceneName != "deck" {
					t.Fatalf("SceneName = %q, want deck", m.gameState.SceneName)
				}
				if got := len(m.gameState.ChatHistory); got != 2 {
					t.Fatalf("ChatHistory len = %d, want 2 (kept local suffix)", got)
				}
				last := m.gameState.ChatHistory[len(m.gameState.ChatHistory)-1]
				if last != user {
					t.Fatalf("local user suffix lost: %+v", last)
				}
				if len(m.pendingUserMessages) != 1 {
					t.Fatalf("pending should stay until server echoes, got %d", len(m.pendingUserMessages))
				}
			},
		},
		{
			name: "history growth resets streaming",
			setup: func(m *ConsoleUI) {
				m.gameState = &state.GameState{ChatHistory: []chat.ChatMessage{assistant}}
				m.isStreaming = true
				m.streamingContent = "partial"
				m.streamingMessageIdx = 0
			},
			server: &state.GameState{ChatHistory: []chat.ChatMessage{assistant, user, followUp}},
			check: func(t *testing.T, m ConsoleUI) {
				if got := len(m.gameState.ChatHistory); got != 3 {
					t.Fatalf("ChatHistory len = %d, want 3", got)
				}
				if m.isStreaming {
					t.Fatal("streaming should reset when history length changes")
				}
				if m.streamingMessageIdx != -1 || m.streamingContent != "" {
					t.Fatalf("streaming fields not cleared: idx=%d content=%q", m.streamingMessageIdx, m.streamingContent)
				}
			},
		},
		{
			name: "pending dropped when server already has the user line",
			setup: func(m *ConsoleUI) {
				m.gameState = &state.GameState{ChatHistory: []chat.ChatMessage{assistant, user}}
				m.pendingUserMessages = []chat.ChatMessage{user}
			},
			server: &state.GameState{ChatHistory: []chat.ChatMessage{assistant, user, followUp}},
			check: func(t *testing.T, m ConsoleUI) {
				if got := len(m.gameState.ChatHistory); got != 3 {
					t.Fatalf("ChatHistory len = %d, want 3", got)
				}
				if len(m.pendingUserMessages) != 0 {
					t.Fatalf("pending should be empty after server echo, got %d", len(m.pendingUserMessages))
				}
			},
		},
		{
			name: "stale player snapshot keeps repeated strike-back suffix",
			setup: func(m *ConsoleUI) {
				m.gameState = &state.GameState{
					Location: "deck",
					ChatHistory: []chat.ChatMessage{
						{Role: "user", Content: "I attack"},
						{Role: "assistant", Content: "You swing."},
						strike,
						{Role: "assistant", Content: "The rat staggers."},
						{Role: "user", Content: "I attack again"},
						{Role: "assistant", Content: "Another blow."},
						strike,
					},
				}
				m.pendingUserMessages = []chat.ChatMessage{strike}
				m.isStreaming = true
				m.streamingContent = "The rat lunges"
				m.streamingMessageIdx = 6
			},
			server: &state.GameState{
				Location: "hold",
				ChatHistory: []chat.ChatMessage{
					{Role: "user", Content: "I attack"},
					{Role: "assistant", Content: "You swing."},
					strike,
					{Role: "assistant", Content: "The rat staggers."},
					{Role: "user", Content: "I attack again"},
					{Role: "assistant", Content: "Another blow."},
				},
			},
			check: func(t *testing.T, m ConsoleUI) {
				if m.gameState.Location != "hold" {
					t.Fatalf("Location = %q, want hold (metadata from server)", m.gameState.Location)
				}
				if got := len(m.gameState.ChatHistory); got != 7 {
					t.Fatalf("ChatHistory len = %d, want 7 (kept local suffix)", got)
				}
				last := m.gameState.ChatHistory[len(m.gameState.ChatHistory)-1]
				if last != strike {
					t.Fatalf("second strike-back dropped: %+v", last)
				}
				if !m.isStreaming || m.streamingContent != "The rat lunges" || m.streamingMessageIdx != 6 {
					t.Fatalf("streaming should stay on prefix keep: streaming=%v idx=%d content=%q",
						m.isStreaming, m.streamingMessageIdx, m.streamingContent)
				}
			},
		},
		{
			name: "occurrence count appends a second copy of a pending user line",
			setup: func(m *ConsoleUI) {
				m.gameState = &state.GameState{ChatHistory: []chat.ChatMessage{assistant}}
				m.pendingUserMessages = []chat.ChatMessage{strike, strike}
			},
			server: &state.GameState{ChatHistory: []chat.ChatMessage{strike, user, followUp}},
			check: func(t *testing.T, m ConsoleUI) {
				if got := len(m.gameState.ChatHistory); got != 4 {
					t.Fatalf("ChatHistory len = %d, want 4 (server + extra strike)", got)
				}
				last := m.gameState.ChatHistory[len(m.gameState.ChatHistory)-1]
				if last != strike {
					t.Fatalf("second strike not appended: %+v", last)
				}
				if len(m.pendingUserMessages) != 1 {
					t.Fatalf("extra strike should stay pending, got %d", len(m.pendingUserMessages))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestUI()
			tt.setup(&m)
			m.mergeServerGameState(tt.server)
			tt.check(t, m)
		})
	}
}
