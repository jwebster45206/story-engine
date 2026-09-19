package llm

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func venicePC() *config.ProviderConfig {
	return &config.ProviderConfig{
		Vendor:       config.VendorVenice,
		APIKey:       "test-key",
		Model:        "test-model",
		BackendModel: "test-backend-model",
	}
}

func TestNewVeniceService(t *testing.T) {
	service := NewVeniceService(venicePC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if service.apiKey != "test-key" {
		t.Errorf("apiKey = %q", service.apiKey)
	}
	if service.modelName != "test-model" {
		t.Errorf("modelName = %q", service.modelName)
	}
	if service.baseURL != veniceBaseURL {
		t.Errorf("baseURL = %q", service.baseURL)
	}
	if service.httpClient == nil {
		t.Error("httpClient nil")
	}
}

func TestVeniceService_ChatStream_SSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		opts, _ := body["stream_options"].(map[string]any)
		if opts == nil || opts["include_usage"] != true {
			t.Error("expected stream_options.include_usage=true")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		responses := []string{
			`data: {"id":"test-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
			`data: {"id":"test-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{"content":" world"}}]}`,
			`data: {"id":"test-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}
		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	svc := NewVeniceService(venicePC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	ch, err := svc.ChatStream(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "Hi"}}, DefaultTemperature)
	require.NoError(t, err)
	var content strings.Builder
	var usage Usage
	for chunk := range ch {
		require.NoError(t, chunk.Error)
		content.WriteString(chunk.Content)
		if chunk.Done {
			usage = chunk.Usage
			break
		}
	}
	assert.Equal(t, "Hello world", content.String())
	assert.Equal(t, 0, usage.InputTokens)
}

func TestVeniceService_ChatStream_UsageAfterFinish(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		responses := []string{
			`data: {"id":"test-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
			`data: {"id":"test-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: {"id":"test-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":3,"total_tokens":12}}`,
			`data: [DONE]`,
		}
		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	svc := NewVeniceService(venicePC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	ch, err := svc.ChatStream(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "Hi"}}, DefaultTemperature)
	require.NoError(t, err)
	var content strings.Builder
	var usage Usage
	for chunk := range ch {
		require.NoError(t, chunk.Error)
		content.WriteString(chunk.Content)
		if chunk.Done {
			usage = chunk.Usage
			break
		}
	}
	assert.Equal(t, "Hello", content.String())
	assert.Equal(t, 9, usage.InputTokens)
	assert.Equal(t, 3, usage.OutputTokens)
	assert.Equal(t, "test-model", usage.Model)
}

func TestVeniceService_DeltaUpdate_JSONSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["response_format"]; !ok {
			t.Error("expected response_format")
		}
		assert.Equal(t, "test-backend-model", body["model"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","model":"test-backend-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"user_location\":\"dock\",\"scene_change\":null,\"item_events\":[],\"npc_events\":[],\"set_vars\":{},\"game_ended\":false}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer server.Close()

	svc := NewVeniceService(venicePC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	delta, usage, err := svc.DeltaUpdate(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "update"}})
	require.NoError(t, err)
	assert.Equal(t, "test-backend-model", usage.Model)
	assert.Equal(t, 5, usage.InputTokens)
	assert.Equal(t, 2, usage.OutputTokens)
	require.NotNil(t, delta)
	assert.Equal(t, "dock", delta.UserLocation)
}

func TestVeniceService_GetRuling_JSONSchema(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","model":"test-backend-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"allowed\":false,\"reasoning\":\"No such exit.\",\"reaction\":\"The wall stops the PC.\",\"scope\":\"movement\",\"focus\":{\"actors\":[],\"locations\":[]}}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":4,"total_tokens":11}}`))
	}))
	defer server.Close()

	svc := NewVeniceService(venicePC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	ruling, usage, err := svc.GetRuling(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "I walk north"}})
	require.NoError(t, err)
	assert.Equal(t, "test-backend-model", got["model"])
	assert.Equal(t, float64(BackendMaxTokens), got["max_tokens"])
	rf, ok := got["response_format"].(map[string]any)
	require.True(t, ok, "expected response_format")
	js, _ := rf["json_schema"].(map[string]any)
	require.NotNil(t, js)
	assert.Equal(t, "ruling", js["name"])
	schema, _ := js["schema"].(map[string]any)
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
	assert.Equal(t, float64(255), maxLen)
	for _, field := range []string{"scope", "focus"} {
		if _, ok := props[field]; !ok {
			t.Errorf("ruling schema missing %q", field)
		}
	}
	if _, ok := props["mentioned"]; ok {
		t.Error("ruling schema should not include mentioned")
	}
	required, _ := schema["required"].([]any)
	gotReq := make([]string, 0, len(required))
	for _, v := range required {
		s, _ := v.(string)
		gotReq = append(gotReq, s)
	}
	for _, field := range []string{"scope", "focus"} {
		if !slices.Contains(gotReq, field) {
			t.Errorf("ruling schema required missing %q: %v", field, gotReq)
		}
	}
	require.NotNil(t, ruling)
	assert.False(t, ruling.Allowed)
	assert.Equal(t, "No such exit.", ruling.Reasoning)
	assert.Equal(t, "The wall stops the PC.", ruling.Reaction)
	assert.Equal(t, chat.RulingScopeMovement, ruling.Scope)
	assert.Equal(t, "test-backend-model", usage.Model)
	assert.Equal(t, 7, usage.InputTokens)
	assert.Equal(t, 4, usage.OutputTokens)
}

func TestVeniceService_ChatStream_PreservesOrderedSystemMessages(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		responses := []string{
			`data: {"id":"test-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
			`data: {"id":"test-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}
		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	svc := NewVeniceService(venicePC(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.baseURL = server.URL
	ch, err := svc.ChatStream(context.Background(), []chat.ChatMessage{
		{Role: chat.ChatRoleSystem, Content: "persistent", IsPersistent: true},
		{Role: chat.ChatRoleSystem, Content: "dynamic"},
		{Role: chat.ChatRoleUser, Content: "Hi"},
	}, DefaultTemperature)
	require.NoError(t, err)
	for chunk := range ch {
		require.NoError(t, chunk.Error)
		if chunk.Done {
			break
		}
	}
	msgs, _ := got["messages"].([]any)
	require.Len(t, msgs, 3)
	first, _ := msgs[0].(map[string]any)
	second, _ := msgs[1].(map[string]any)
	third, _ := msgs[2].(map[string]any)
	assert.Equal(t, "system", first["role"])
	assert.Equal(t, "persistent", first["content"])
	assert.Equal(t, "system", second["role"])
	assert.Equal(t, "dynamic", second["content"])
	assert.Equal(t, "user", third["role"])
	assert.Equal(t, "Hi", third["content"])
	_, hasCache := first["cache_control"]
	assert.False(t, hasCache)
}

func TestVeniceStreamResponseParsing(t *testing.T) {
	streamData := `{"id":"test-1","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello world"},"finish_reason":null}]}`
	var streamResp VeniceStreamResponse
	require.NoError(t, json.Unmarshal([]byte(streamData), &streamResp))
	assert.Equal(t, "Hello world", streamResp.Choices[0].Delta.Content)
}
