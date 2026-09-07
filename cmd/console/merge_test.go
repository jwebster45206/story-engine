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
			name: "pending user not on server is appended then cleared from pending",
			setup: func(m *ConsoleUI) {
				m.gameState = &state.GameState{ChatHistory: []chat.ChatMessage{assistant, user}}
				m.pendingUserMessages = []chat.ChatMessage{user}
			},
			server: &state.GameState{
				SceneName:   "deck",
				ChatHistory: []chat.ChatMessage{assistant},
			},
			check: func(t *testing.T, m ConsoleUI) {
				if got := len(m.gameState.ChatHistory); got != 2 {
					t.Fatalf("ChatHistory len = %d, want 2 (server + pending)", got)
				}
				last := m.gameState.ChatHistory[len(m.gameState.ChatHistory)-1]
				if last != user {
					t.Fatalf("pending user not appended: %+v", last)
				}
				if len(m.pendingUserMessages) != 0 {
					t.Fatalf("pending is cleared after merge into history, got %d", len(m.pendingUserMessages))
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
