package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestFeedSSELine_CompletesEvent(t *testing.T) {
	cur, done := feedSSELine(SSEEvent{}, "event: chat.chunk")
	if done != nil {
		t.Fatal("event line should not complete a frame")
	}
	cur, done = feedSSELine(cur, `data: {"content":"Hi"}`)
	if done != nil {
		t.Fatal("data line should not complete a frame")
	}
	if cur.Type != "chat.chunk" {
		t.Fatalf("Type = %q, want chat.chunk", cur.Type)
	}
	if cur.Data["content"] != "Hi" {
		t.Fatalf("Data content = %v, want Hi", cur.Data["content"])
	}

	cur, done = feedSSELine(cur, "")
	if done == nil {
		t.Fatal("blank line should complete the event")
	}
	if done.Type != "chat.chunk" {
		t.Fatalf("completed Type = %q", done.Type)
	}
	if cur.Type != "" {
		t.Fatalf("accumulator should reset, Type = %q", cur.Type)
	}
}

func TestFeedSSELine_KeepaliveIgnored(t *testing.T) {
	cur, done := feedSSELine(SSEEvent{}, ": keepalive")
	if done != nil {
		t.Fatal("keepalive comment should not emit an event")
	}
	if cur.Type != "" || cur.Data != nil {
		t.Fatalf("keepalive mutated accumulator: %+v", cur)
	}
	_, done = feedSSELine(cur, "")
	if done != nil {
		t.Fatal("blank after keepalive with empty type should not emit")
	}
}

func TestFeedSSELine_InvalidJSONIgnored(t *testing.T) {
	cur, _ := feedSSELine(SSEEvent{}, "event: request.failed")
	cur, done := feedSSELine(cur, "data: not-json")
	if done != nil {
		t.Fatal("bad data should not complete")
	}
	if cur.Data != nil {
		t.Fatalf("invalid JSON should leave Data unset, got %v", cur.Data)
	}
	_, done = feedSSELine(cur, "")
	if done == nil || done.Type != "request.failed" {
		t.Fatalf("event should still complete with type, got %+v", done)
	}
}

func TestParseSSEStream(t *testing.T) {
	body := strings.Join([]string{
		"event: request.processing",
		`data: {"user_message":"look"}`,
		"",
		": keepalive",
		"",
		"event: chat.chunk",
		`data: {"content":"Ahoy"}`,
		"",
	}, "\n") + "\n"

	gameID := uuid.New()
	ch := make(chan SSEEvent, 8)
	if err := parseSSEStream(context.Background(), strings.NewReader(body), gameID, ch); err != nil {
		t.Fatalf("parseSSEStream: %v", err)
	}
	close(ch)

	var got []SSEEvent
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (keepalive omitted): %+v", len(got), got)
	}
	if got[0].Type != "request.processing" || got[0].Data["user_message"] != "look" {
		t.Fatalf("first event = %+v", got[0])
	}
	if got[1].Type != "chat.chunk" || got[1].Data["content"] != "Ahoy" {
		t.Fatalf("second event = %+v", got[1])
	}
	for i, ev := range got {
		if ev.GameID != gameID {
			t.Fatalf("event %d GameID = %v, want %v", i, ev.GameID, gameID)
		}
	}
}

func TestParseSSEStream_CancelUnblocksSend(t *testing.T) {
	body := strings.Join([]string{
		"event: chat.chunk",
		`data: {"content":"Hi"}`,
		"",
	}, "\n") + "\n"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan SSEEvent) // unbuffered; nobody receives, so send blocks

	errCh := make(chan error, 1)
	go func() {
		errCh <- parseSSEStream(ctx, strings.NewReader(body), uuid.New(), ch)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parseSSEStream hung after cancel")
	}
}

// hangReader yields data then blocks until ctx is canceled.
type hangReader struct {
	ctx  context.Context
	data []byte
}

func (h *hangReader) Read(p []byte) (int, error) {
	if len(h.data) > 0 {
		n := copy(p, h.data)
		h.data = h.data[n:]
		return n, nil
	}
	<-h.ctx.Done()
	return 0, h.ctx.Err()
}

func TestParseSSEStream_CancelMidStream(t *testing.T) {
	gameID := uuid.New()
	body := strings.Join([]string{
		"event: chat.chunk",
		`data: {"content":"Hi"}`,
		"",
	}, "\n") + "\n"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &hangReader{ctx: ctx, data: []byte(body)}
	ch := make(chan SSEEvent, 1)

	errCh := make(chan error, 1)
	go func() {
		errCh <- parseSSEStream(ctx, r, gameID, ch)
	}()

	select {
	case ev := <-ch:
		if ev.GameID != gameID {
			t.Fatalf("GameID = %v, want %v", ev.GameID, gameID)
		}
		if ev.Type != "chat.chunk" {
			t.Fatalf("Type = %q, want chat.chunk", ev.Type)
		}
	case err := <-errCh:
		t.Fatalf("parseSSEStream returned before event: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parseSSEStream hung after cancel")
	}
}

func TestConsumeSSEEvents_ClosedChannel(t *testing.T) {
	id := uuid.New()
	m := newTestUI()
	m.sseGameID = id
	ch := make(chan SSEEvent)
	close(ch)

	msg := m.consumeSSEEvents(ch)()
	got, ok := msg.(sseDisconnectedMsg)
	if !ok {
		t.Fatalf("got %T, want sseDisconnectedMsg", msg)
	}
	if got.gameID != id {
		t.Fatalf("gameID = %v, want %v", got.gameID, id)
	}
}

func TestConsumeSSEEvents_Event(t *testing.T) {
	id := uuid.New()
	m := newTestUI()
	m.sseGameID = id
	ch := make(chan SSEEvent, 1)
	ch <- SSEEvent{Type: "connected", GameID: id}

	msg := m.consumeSSEEvents(ch)()
	got, ok := msg.(sseEventMsg)
	if !ok {
		t.Fatalf("got %T, want sseEventMsg", msg)
	}
	if got.event.Type != "connected" {
		t.Fatalf("Type = %q, want connected", got.event.Type)
	}
}

func TestUpdate_SSEDisconnectedUnsticksLoading(t *testing.T) {
	id := uuid.New()
	m := newTestUI()
	m.showScenarioModal = false
	m.sseGameID = id
	m.loading = true
	m.isStreaming = true
	m.gameState = &state.GameState{ID: id}

	model, cmd := m.Update(sseDisconnectedMsg{gameID: id})
	ui := model.(ConsoleUI)
	if ui.loading || ui.isStreaming {
		t.Fatal("loading/isStreaming should clear so Enter works again")
	}
	if cmd == nil {
		t.Fatal("expected reconnect tick")
	}
	if len(ui.gameState.ChatHistory) != 1 {
		t.Fatalf("ChatHistory len = %d, want 1 system error", len(ui.gameState.ChatHistory))
	}
	if ui.gameState.ChatHistory[0].Role != "system" {
		t.Fatalf("role = %q, want system", ui.gameState.ChatHistory[0].Role)
	}
	if !strings.Contains(ui.gameState.ChatHistory[0].Content, "lost connection") {
		t.Fatalf("content = %q, want lost connection", ui.gameState.ChatHistory[0].Content)
	}
}

func TestUpdate_SSEDisconnectedIdleIsSilent(t *testing.T) {
	id := uuid.New()
	m := newTestUI()
	m.showScenarioModal = false
	m.sseGameID = id
	m.gameState = &state.GameState{ID: id}

	model, cmd := m.Update(sseDisconnectedMsg{gameID: id})
	ui := model.(ConsoleUI)
	if cmd == nil {
		t.Fatal("expected reconnect tick")
	}
	if len(ui.gameState.ChatHistory) != 0 {
		t.Fatalf("idle disconnect should not append chat, got %d", len(ui.gameState.ChatHistory))
	}
}

func TestUpdate_SSEDisconnectedIgnoresStaleGame(t *testing.T) {
	id := uuid.New()
	m := newTestUI()
	m.showScenarioModal = false
	m.sseGameID = id
	m.loading = true
	m.gameState = &state.GameState{ID: id}

	model, cmd := m.Update(sseDisconnectedMsg{gameID: uuid.New()})
	ui := model.(ConsoleUI)
	if !ui.loading {
		t.Fatal("stale disconnect should not clear loading")
	}
	if cmd != nil {
		t.Fatal("stale disconnect should not reconnect")
	}
}

func TestUpdate_SSEDisconnectedDuringQuitModal(t *testing.T) {
	id := uuid.New()
	m := newTestUI()
	m.showScenarioModal = false
	m.showQuitModal = true
	m.sseGameID = id
	m.loading = true
	m.gameState = &state.GameState{ID: id}

	model, cmd := m.Update(sseDisconnectedMsg{gameID: id})
	ui := model.(ConsoleUI)
	if ui.loading {
		t.Fatal("quit modal must not swallow SSE disconnect")
	}
	if cmd == nil {
		t.Fatal("expected reconnect tick")
	}
}

func TestUpdate_SSEReconnectSkippedWithoutGame(t *testing.T) {
	id := uuid.New()
	m := newTestUI()
	m.showScenarioModal = false
	m.gameState = nil

	_, cmd := m.Update(sseReconnectMsg{gameID: id})
	if cmd != nil {
		t.Fatal("reconnect without a game should be a no-op")
	}
}
