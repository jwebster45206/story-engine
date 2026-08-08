package services

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
		expectedSystem         string
		expectedNonSystemCount int
	}{
		{
			name: "single system message",
			messages: []chat.ChatMessage{
				{Role: chat.ChatRoleSystem, Content: "You are a helpful assistant."},
				{Role: chat.ChatRoleUser, Content: "Hello"},
				{Role: chat.ChatRoleAgent, Content: "Hi there!"},
			},
			expectedSystem:         "You are a helpful assistant.",
			expectedNonSystemCount: 2,
		},
		{
			name: "multiple system messages",
			messages: []chat.ChatMessage{
				{Role: chat.ChatRoleSystem, Content: "You are a helpful assistant."},
				{Role: chat.ChatRoleUser, Content: "Hello"},
				{Role: chat.ChatRoleSystem, Content: "Be concise."},
				{Role: chat.ChatRoleAgent, Content: "Hi there!"},
			},
			expectedSystem:         "You are a helpful assistant.\n\nBe concise.",
			expectedNonSystemCount: 2,
		},
		{
			name: "no system messages",
			messages: []chat.ChatMessage{
				{Role: chat.ChatRoleUser, Content: "Hello"},
				{Role: chat.ChatRoleAgent, Content: "Hi there!"},
			},
			expectedSystem:         "",
			expectedNonSystemCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			systemPrompt, nonSystemMessages := service.splitChatMessages(tt.messages)
			if systemPrompt != tt.expectedSystem {
				t.Errorf("system = %q, want %q", systemPrompt, tt.expectedSystem)
			}
			if len(nonSystemMessages) != tt.expectedNonSystemCount {
				t.Errorf("non-system count = %d", len(nonSystemMessages))
			}
		})
	}
}

func TestAnthropicService_Chat_RequestShape(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing x-api-key")
		}
		if r.Header.Get("anthropic-version") != anthropicVersion {
			t.Errorf("anthropic-version = %s", r.Header.Get("anthropic-version"))
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hi"}],"model":"claude-test","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	svc := NewAnthropicService(anthropicPC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	resp, err := svc.Chat(context.Background(), []chat.ChatMessage{
		{Role: chat.ChatRoleSystem, Content: "sys"},
		{Role: chat.ChatRoleUser, Content: "Hello", IsStoryEvent: true},
	}, 0.7)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message != "hi" {
		t.Fatalf("message = %q", resp.Message)
	}
	if gotBody["model"] != "claude-test" {
		t.Fatalf("model = %v", gotBody["model"])
	}
	if _, ok := gotBody["temperature"]; ok {
		t.Fatal("temperature must not be sent to Anthropic")
	}
	if gotBody["system"] != "sys" {
		t.Fatalf("system = %v", gotBody["system"])
	}
	raw, _ := json.Marshal(gotBody)
	if strings.Contains(string(raw), "is_story_event") {
		t.Fatalf("is_story_event leaked: %s", raw)
	}
}

func TestAnthropicService_ChatStream_SSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		frames := []string{
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}`,
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
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatal(chunk.Error)
		}
		content.WriteString(chunk.Content)
		if chunk.Done {
			break
		}
	}
	if content.String() != "Hello world" {
		t.Fatalf("content = %q", content.String())
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
	delta, model, err := svc.DeltaUpdate(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "update"}})
	if err != nil {
		t.Fatal(err)
	}
	if model != "claude-backend" {
		t.Fatalf("model = %q", model)
	}
	if delta == nil || delta.UserLocation != "dock" {
		t.Fatalf("delta = %#v", delta)
	}
}
