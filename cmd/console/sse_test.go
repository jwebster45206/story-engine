package main

import (
	"context"
	"strings"
	"testing"
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

	ch := make(chan SSEEvent, 8)
	if err := parseSSEStream(context.Background(), strings.NewReader(body), ch); err != nil {
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
}
