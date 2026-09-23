package llm

import (
	"context"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

type recordingLLM struct {
	name     string
	streamed bool
	ruled    bool
	reduced  bool
}

func (r *recordingLLM) ChatStream(_ context.Context, _ []chat.ChatMessage, _ float64) (<-chan StreamChunk, error) {
	r.streamed = true
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Content: r.name, Done: true, Usage: Usage{Vendor: r.name, Model: r.name}}
	close(ch)
	return ch, nil
}

func (r *recordingLLM) GetRuling(_ context.Context, _ []chat.ChatMessage) (*chat.Ruling, Usage, error) {
	r.ruled = true
	return &chat.Ruling{Allowed: true, Reasoning: r.name}, Usage{Vendor: r.name, Model: r.name}, nil
}

func (r *recordingLLM) DeltaUpdate(_ context.Context, _ []chat.ChatMessage) (*conditionals.GameStateDelta, Usage, error) {
	r.reduced = true
	return &conditionals.GameStateDelta{UserLocation: r.name}, Usage{Vendor: r.name, Model: r.name}, nil
}

func TestSplitService_RoutesByRole(t *testing.T) {
	narrator := &recordingLLM{name: "venice"}
	backend := &recordingLLM{name: "anthropic"}
	svc := &splitService{narrator: narrator, backend: backend}

	ch, err := svc.ChatStream(context.Background(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	chunk := <-ch
	if chunk.Content != "venice" {
		t.Fatalf("stream content = %q", chunk.Content)
	}
	if !narrator.streamed || backend.streamed {
		t.Fatalf("stream routing: narrator=%v backend=%v", narrator.streamed, backend.streamed)
	}

	ruling, usage, err := svc.GetRuling(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if ruling.Reasoning != "anthropic" || usage.Vendor != "anthropic" {
		t.Fatalf("ruling = %#v usage=%+v", ruling, usage)
	}
	if narrator.ruled || !backend.ruled {
		t.Fatalf("ruling routing: narrator=%v backend=%v", narrator.ruled, backend.ruled)
	}

	delta, usage, err := svc.DeltaUpdate(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if delta.UserLocation != "anthropic" || usage.Vendor != "anthropic" {
		t.Fatalf("delta = %#v usage=%+v", delta, usage)
	}
	if narrator.reduced || !backend.reduced {
		t.Fatalf("delta routing: narrator=%v backend=%v", narrator.reduced, backend.reduced)
	}
}
