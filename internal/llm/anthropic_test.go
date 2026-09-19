package llm

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/pkg/chat"
)

func anthropicPC() *config.ProviderConfig {
	return &config.ProviderConfig{
		Vendor:       config.VendorAnthropic,
		APIKey:       "test-key",
		Model:        "claude-test",
		BackendModel: "claude-backend",
	}
}

func TestNewAnthropicService(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewAnthropicService(anthropicPC(), log)
	if service.apiKey != "test-key" {
		t.Errorf("apiKey = %q", service.apiKey)
	}
	if service.modelName != "claude-test" {
		t.Errorf("modelName = %q", service.modelName)
	}
	if service.baseURL != anthropicBaseURL {
		t.Errorf("baseURL = %q", service.baseURL)
	}
	if service.httpClient == nil {
		t.Error("httpClient nil")
	}
}

func TestAnthropicService_ExtractSystemMessage(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewAnthropicService(anthropicPC(), log)

	tests := []struct {
		name                   string
		messages               []chat.ChatMessage
		expectedBlocks         []AnthropicSystemBlock
		expectedNonSystemCount int
	}{
		{
			name: "single system message",
			messages: []chat.ChatMessage{
				{Role: chat.ChatRoleSystem, Content: "You are a helpful assistant."},
				{Role: chat.ChatRoleUser, Content: "Hello"},
				{Role: chat.ChatRoleAgent, Content: "Hi there!"},
			},
			expectedBlocks: []AnthropicSystemBlock{
				{Type: "text", Text: "You are a helpful assistant."},
			},
			expectedNonSystemCount: 2,
		},
		{
			name: "persistent then dynamic system messages",
			messages: []chat.ChatMessage{
				{Role: chat.ChatRoleSystem, Content: "You are a helpful assistant.", IsPersistent: true},
				{Role: chat.ChatRoleUser, Content: "Hello"},
				{Role: chat.ChatRoleSystem, Content: "Be concise."},
				{Role: chat.ChatRoleAgent, Content: "Hi there!"},
			},
			expectedBlocks: []AnthropicSystemBlock{
				{
					Type: "text",
					Text: "You are a helpful assistant.",
					CacheControl: &AnthropicCacheControl{
						Type: "ephemeral",
						TTL:  anthropicCacheTTLHour,
					},
				},
				{Type: "text", Text: "Be concise."},
			},
			expectedNonSystemCount: 2,
		},
		{
			name: "no system messages",
			messages: []chat.ChatMessage{
				{Role: chat.ChatRoleUser, Content: "Hello"},
				{Role: chat.ChatRoleAgent, Content: "Hi there!"},
			},
			expectedBlocks:         nil,
			expectedNonSystemCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			systemBlocks, nonSystemMessages := service.splitChatMessages(tt.messages)
			if len(systemBlocks) != len(tt.expectedBlocks) {
				t.Fatalf("system blocks = %#v, want %#v", systemBlocks, tt.expectedBlocks)
			}
			for i, got := range systemBlocks {
				want := tt.expectedBlocks[i]
				if got.Type != want.Type || got.Text != want.Text {
					t.Errorf("block %d = %#v, want %#v", i, got, want)
				}
				if want.CacheControl == nil {
					if got.CacheControl != nil {
						t.Errorf("block %d cache_control = %#v, want nil", i, got.CacheControl)
					}
					continue
				}
				if got.CacheControl == nil || got.CacheControl.Type != want.CacheControl.Type || got.CacheControl.TTL != want.CacheControl.TTL {
					t.Errorf("block %d cache_control = %#v, want %#v", i, got.CacheControl, want.CacheControl)
				}
			}
			if len(nonSystemMessages) != tt.expectedNonSystemCount {
				t.Errorf("non-system count = %d", len(nonSystemMessages))
			}
		})
	}
}

func TestAnthropicService_ChatStream_SSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		frames := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-test","usage":{"input_tokens":12,"output_tokens":0}}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}
		for _, f := range frames {
			_, _ = w.Write([]byte(f + "\n"))
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		}
	}))
	defer server.Close()

	svc := NewAnthropicService(anthropicPC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	ch, err := svc.ChatStream(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "Hi"}}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	var usage Usage
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatal(chunk.Error)
		}
		content.WriteString(chunk.Content)
		if chunk.Done {
			usage = chunk.Usage
			break
		}
	}
	if content.String() != "Hello world" {
		t.Fatalf("content = %q", content.String())
	}
	if usage.InputTokens != 12 || usage.OutputTokens != 4 || usage.Model != "claude-test" {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestAnthropicService_DeltaUpdate_ToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["tools"]; !ok {
			t.Error("expected tools in delta request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"apply_changes","input":{"user_location":"dock","scene_change":null,"item_events":[],"npc_events":[],"set_vars":{},"game_ended":false}}],"model":"claude-backend","stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	svc := NewAnthropicService(anthropicPC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	delta, usage, err := svc.DeltaUpdate(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "update"}})
	if err != nil {
		t.Fatal(err)
	}
	if usage.Model != "claude-backend" {
		t.Fatalf("model = %q", usage.Model)
	}
	if usage.InputTokens != 1 || usage.OutputTokens != 1 {
		t.Fatalf("usage = %+v", usage)
	}
	if delta == nil || delta.UserLocation != "dock" {
		t.Fatalf("delta = %#v", delta)
	}
}

func TestAnthropicService_GetRuling_ToolUse(t *testing.T) {
	var gotModel string
	var gotMaxTokens float64
	var tools []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel, _ = body["model"].(string)
		gotMaxTokens, _ = body["max_tokens"].(float64)
		tools, _ = body["tools"].([]any)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"ruling","input":{"allowed":false,"reasoning":"No such exit.","reaction":"The wall stops the PC.","scope":"movement","focus":{"npcs":[],"locations":[]}}}],"model":"claude-backend","stop_reason":"tool_use","usage":{"input_tokens":8,"output_tokens":3}}`))
	}))
	defer server.Close()

	svc := NewAnthropicService(anthropicPC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	ruling, usage, err := svc.GetRuling(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "I walk north"}})
	if err != nil {
		t.Fatal(err)
	}
	if gotModel != "claude-backend" {
		t.Fatalf("model = %q", gotModel)
	}
	if int(gotMaxTokens) != BackendMaxTokens {
		t.Fatalf("max_tokens = %v, want %d", gotMaxTokens, BackendMaxTokens)
	}
	if len(tools) != 1 {
		t.Fatal("expected ruling tool")
	}
	tool, _ := tools[0].(map[string]any)
	if tool["name"] != "ruling" {
		t.Fatalf("tool name = %v", tool["name"])
	}
	schema, _ := tool["input_schema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	reasoning, _ := props["reasoning"].(map[string]any)
	anyOf, _ := reasoning["anyOf"].([]any)
	maxLen := 0.0
	for _, opt := range anyOf {
		m, _ := opt.(map[string]any)
		if m["type"] == "string" {
			if ml, ok := m["maxLength"].(float64); ok {
				maxLen = ml
			}
		}
	}
	if maxLen != 255 {
		t.Fatalf("reasoning maxLength = %v, want 255", maxLen)
	}
	for _, field := range []string{"scope", "focus"} {
		if _, ok := props[field]; !ok {
			t.Errorf("ruling schema missing %q", field)
		}
	}
	required, _ := schema["required"].([]any)
	gotReq := make(map[string]bool, len(required))
	for _, v := range required {
		s, _ := v.(string)
		gotReq[s] = true
	}
	for _, field := range []string{"scope", "focus"} {
		if !gotReq[field] {
			t.Errorf("ruling schema required missing %q: %v", field, required)
		}
	}
	if ruling == nil || ruling.Allowed || ruling.Reasoning != "No such exit." || ruling.Reaction != "The wall stops the PC." || ruling.Scope != chat.RulingScopeMovement {
		t.Fatalf("ruling = %#v", ruling)
	}
	if usage.Model != "claude-backend" || usage.InputTokens != 8 || usage.OutputTokens != 3 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestAnthropicService_ChatStream_PromptCache(t *testing.T) {
	var gotBody map[string]any
	var gotBeta string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBeta = r.Header.Get("anthropic-beta")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		frames := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-test","usage":{"input_tokens":12,"output_tokens":0,"cache_creation_input_tokens":80,"cache_read_input_tokens":40}}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"ok"}}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}
		for _, f := range frames {
			_, _ = w.Write([]byte(f + "\n"))
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		}
	}))
	defer server.Close()

	svc := NewAnthropicService(anthropicPC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	ch, err := svc.ChatStream(context.Background(), []chat.ChatMessage{
		{Role: chat.ChatRoleSystem, Content: "persistent", IsPersistent: true},
		{Role: chat.ChatRoleSystem, Content: "dynamic"},
		{Role: chat.ChatRoleUser, Content: "Hi"},
	}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	var usage Usage
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatal(chunk.Error)
		}
		if chunk.Done {
			usage = chunk.Usage
			break
		}
	}
	if usage.CacheCreationInputTokens != 80 || usage.CacheReadInputTokens != 40 {
		t.Fatalf("cache usage = %+v", usage)
	}
	if gotBeta != anthropicCacheTTLBeta {
		t.Fatalf("anthropic-beta = %q, want %q", gotBeta, anthropicCacheTTLBeta)
	}
	system, _ := gotBody["system"].([]any)
	if len(system) != 2 {
		t.Fatalf("system blocks = %#v", gotBody["system"])
	}
	first, _ := system[0].(map[string]any)
	if first["text"] != "persistent" {
		t.Fatalf("first system text = %#v", first)
	}
	cache, _ := first["cache_control"].(map[string]any)
	if cache["type"] != "ephemeral" || cache["ttl"] != anthropicCacheTTLHour {
		t.Fatalf("cache_control = %#v", cache)
	}
	second, _ := system[1].(map[string]any)
	if second["text"] != "dynamic" {
		t.Fatalf("second system text = %#v", second)
	}
	if _, ok := second["cache_control"]; ok {
		t.Fatalf("dynamic block should not have cache_control: %#v", second)
	}
}
