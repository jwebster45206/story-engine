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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func venicePC(baseURL string) *config.ProviderConfig {
	return &config.ProviderConfig{
		Vendor:       config.VendorVenice,
		APIKey:       "test-key",
		Model:        "test-model",
		BackendModel: "test-backend-model",
		BaseURL:      baseURL,
	}
}

func TestNewVeniceService(t *testing.T) {
	service := NewVeniceService(venicePC(""), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if service.apiKey != "test-key" {
		t.Errorf("apiKey = %q", service.apiKey)
	}
	if service.modelName != "test-model" {
		t.Errorf("modelName = %q", service.modelName)
	}
	if service.baseURL != defaultVeniceBaseURL {
		t.Errorf("baseURL = %q", service.baseURL)
	}
	if service.httpClient == nil {
		t.Error("httpClient nil")
	}
}

func TestVeniceService_Chat_RequestShape(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Authorization = %s", r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	svc := NewVeniceService(venicePC(server.URL), slog.New(slog.NewTextHandler(io.Discard, nil)))
	resp, err := svc.Chat(context.Background(), []chat.ChatMessage{
		{Role: chat.ChatRoleUser, Content: "Hello", IsStoryEvent: true},
	}, 0.7)
	require.NoError(t, err)
	assert.Equal(t, "hi", resp.Message)
	assert.Equal(t, "test-model", gotBody["model"])
	assert.EqualValues(t, 0.7, gotBody["temperature"])
	raw, _ := json.Marshal(gotBody)
	assert.NotContains(t, string(raw), "is_story_event")
	vp, ok := gotBody["venice_parameters"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, vp["include_venice_system_prompt"])
}

func TestVeniceService_ChatStream_SSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	svc := NewVeniceService(venicePC(server.URL), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ch, err := svc.ChatStream(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "Hi"}}, DefaultTemperature)
	require.NoError(t, err)
	var content strings.Builder
	for chunk := range ch {
		require.NoError(t, chunk.Error)
		content.WriteString(chunk.Content)
		if chunk.Done {
			break
		}
	}
	assert.Equal(t, "Hello world", content.String())
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
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"{\"user_location\":\"dock\",\"scene_change\":null,\"item_events\":[],\"npc_events\":[],\"set_vars\":{},\"game_ended\":false}"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	svc := NewVeniceService(venicePC(server.URL), slog.New(slog.NewTextHandler(io.Discard, nil)))
	delta, model, err := svc.DeltaUpdate(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "update"}})
	require.NoError(t, err)
	assert.Equal(t, "test-backend-model", model)
	require.NotNil(t, delta)
	assert.Equal(t, "dock", delta.UserLocation)
}

func TestVeniceStreamResponseParsing(t *testing.T) {
	streamData := `{"id":"test-1","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello world"},"finish_reason":null}]}`
	var streamResp VeniceStreamResponse
	require.NoError(t, json.Unmarshal([]byte(streamData), &streamResp))
	assert.Equal(t, "Hello world", streamResp.Choices[0].Delta.Content)
}
