package main

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func newTestUI() ConsoleUI {
	return NewConsoleUI(&ConsoleConfig{}, nil)
}

func TestMergeServerGameState_NilServer(t *testing.T) {
	m := newTestUI()
	gs := &state.GameState{SceneName: "dock"}
	m.gameState = gs
	m.mergeServerGameState(nil)
	if m.gameState != gs {
		t.Fatal("nil server should leave local game state unchanged")
	}
}

func TestMergeServerGameState_AssignsWhenEmpty(t *testing.T) {
	m := newTestUI()
	server := &state.GameState{
		SceneName: "hold",
		ChatHistory: []chat.ChatMessage{
			{Role: "assistant", Content: "Welcome."},
		},
	}
	m.mergeServerGameState(server)
	if m.gameState != server {
		t.Fatal("empty local state should take the server pointer")
	}
}

func TestMergeServerGameState_KeepsPendingUser(t *testing.T) {
	m := newTestUI()
	pending := chat.ChatMessage{Role: "user", Content: "look around"}
	m.gameState = &state.GameState{
		ChatHistory: []chat.ChatMessage{
			{Role: "assistant", Content: "You are on deck."},
			pending,
		},
	}
	m.pendingUserMessages = []chat.ChatMessage{pending}
	m.isStreaming = true
	m.streamingContent = "partial"
	m.streamingMessageIdx = 1

	server := &state.GameState{
		SceneName: "deck",
		ChatHistory: []chat.ChatMessage{
			{Role: "assistant", Content: "You are on deck."},
		},
	}
	m.mergeServerGameState(server)

	if got := len(m.gameState.ChatHistory); got != 2 {
		t.Fatalf("ChatHistory len = %d, want 2 (server + pending)", got)
	}
	last := m.gameState.ChatHistory[len(m.gameState.ChatHistory)-1]
	if last.Role != "user" || last.Content != pending.Content {
		t.Fatalf("pending user not appended: %+v", last)
	}
	if len(m.pendingUserMessages) != 0 {
		t.Fatalf("pending is cleared after it is merged into history, got %d", len(m.pendingUserMessages))
	}
}

func TestMergeServerGameState_ResetsStreamingOnHistoryGrowth(t *testing.T) {
	m := newTestUI()
	m.gameState = &state.GameState{
		ChatHistory: []chat.ChatMessage{
			{Role: "assistant", Content: "You are on deck."},
		},
	}
	m.isStreaming = true
	m.streamingContent = "partial"
	m.streamingMessageIdx = 0

	server := &state.GameState{
		ChatHistory: []chat.ChatMessage{
			{Role: "assistant", Content: "You are on deck."},
			{Role: "user", Content: "look around"},
			{Role: "assistant", Content: "Waves crash."},
		},
	}
	m.mergeServerGameState(server)

	if got := len(m.gameState.ChatHistory); got != 3 {
		t.Fatalf("ChatHistory len = %d, want 3", got)
	}
	if m.isStreaming {
		t.Fatal("streaming should reset when history length changes")
	}
	if m.streamingMessageIdx != -1 || m.streamingContent != "" {
		t.Fatalf("streaming fields not cleared: idx=%d content=%q", m.streamingMessageIdx, m.streamingContent)
	}
}

func TestMergeServerGameState_DropsPendingWhenServerHasIt(t *testing.T) {
	m := newTestUI()
	user := chat.ChatMessage{Role: "user", Content: "look around"}
	m.gameState = &state.GameState{
		ChatHistory: []chat.ChatMessage{
			{Role: "assistant", Content: "You are on deck."},
			user,
		},
	}
	m.pendingUserMessages = []chat.ChatMessage{user}

	server := &state.GameState{
		ChatHistory: []chat.ChatMessage{
			{Role: "assistant", Content: "You are on deck."},
			user,
			{Role: "assistant", Content: "Waves crash."},
		},
	}
	m.mergeServerGameState(server)

	if got := len(m.gameState.ChatHistory); got != 3 {
		t.Fatalf("ChatHistory len = %d, want 3", got)
	}
	if len(m.pendingUserMessages) != 0 {
		t.Fatalf("pending should be empty after server echo, got %d", len(m.pendingUserMessages))
	}
}
