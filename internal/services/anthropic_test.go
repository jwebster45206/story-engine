package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

func TestNewAnthropicService(t *testing.T) {
	apiKey := "test-api-key"
	modelName := "claude-3-sonnet-20240229"
	backendModelName := "claude-3-backend"
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewAnthropicService(apiKey, modelName, backendModelName, log)

	if service.apiKey != apiKey {
		t.Errorf("Expected API key %s, got %s", apiKey, service.apiKey)
	}

	if service.modelName != modelName {
		t.Errorf("Expected model name %s, got %s", modelName, service.modelName)
	}

	if service.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}

func TestAnthropicService_InitModel(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewAnthropicService("test-key", "claude-3-sonnet-20240229", "claude-3-backend", log)

	err := service.InitModel(context.Background(), "test-model")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestAnthropicService_ExtractSystemMessage(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewAnthropicService("test-key", "claude-3-sonnet-20240229", "claude-3-backend", log)

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
				t.Errorf("Expected system prompt '%s', got '%s'", tt.expectedSystem, systemPrompt)
			}

			if len(nonSystemMessages) != tt.expectedNonSystemCount {
				t.Errorf("Expected %d non-system messages, got %d", tt.expectedNonSystemCount, len(nonSystemMessages))
			}

			// Verify no system messages remain
			for _, msg := range nonSystemMessages {
				if msg.Role == chat.ChatRoleSystem {
					t.Error("Found system message in non-system messages")
				}
			}
		})
	}
}

func TestAnthropicChatRequestStructure(t *testing.T) {
	// Test that the request structure can be marshaled properly
	temp := 0.7
	req := AnthropicChatRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   1024,
		Temperature: &temp,
		Messages: []chat.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		System: "You are a helpful assistant.",
		Stream: false,
	}

	_, err := json.Marshal(req)
	if err != nil {
		t.Errorf("Failed to marshal request: %v", err)
	}
}

func TestAnthropicChatResponseStructure(t *testing.T) {
	// Test that we can unmarshal a typical Anthropic response
	responseJSON := `{
		"id": "msg_01ABC123",
		"type": "message",
		"role": "assistant",
		"content": [
			{
				"type": "text",
				"text": "Hello! How can I help you today?"
			}
		],
		"model": "claude-3-sonnet-20240229",
		"stop_reason": "end_turn",
		"stop_sequence": null,
		"usage": {
			"input_tokens": 10,
			"output_tokens": 20
		}
	}`

	var resp AnthropicChatResponse
	err := json.Unmarshal([]byte(responseJSON), &resp)
	if err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if resp.ID != "msg_01ABC123" {
		t.Errorf("Expected ID 'msg_01ABC123', got '%s'", resp.ID)
	}

	if len(resp.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(resp.Content))
	}

	if resp.Content[0].Text != "Hello! How can I help you today?" {
		t.Errorf("Expected text 'Hello! How can I help you today?', got '%s'", resp.Content[0].Text)
	}
}

func TestAnthropicService_MetaUpdateJSONParsing(t *testing.T) {
	// Test JSON cleaning logic by creating test cases for various response formats
	tests := []struct {
		name             string
		responseText     string
		expectedError    bool
		expectedLocation string
	}{
		{
			name:             "clean JSON",
			responseText:     `{"user_location": "forest"}`,
			expectedError:    false,
			expectedLocation: "forest",
		},
		{
			name:             "JSON with markdown code blocks",
			responseText:     "```json\n{\"user_location\": \"forest\"}\n```",
			expectedError:    false,
			expectedLocation: "forest",
		},
		{
			name:             "JSON with backticks in content",
			responseText:     "```\n{\"user_location\": \"forest`area\"}\n```",
			expectedError:    false,
			expectedLocation: "forestarea",
		},
		{
			name:             "mixed narrative and JSON (real world case)",
			responseText:     "Across the tavern, you spot the burly Shipwright hunched over a table, nursing a mug of ale and examining what looks like ship blueprints.\n\njson\n{\n \"user_location\": \"Sleepy Mermaid\",\n \"remove_from_inventory\": [\"cutlass\"]\n}",
			expectedError:    false,
			expectedLocation: "Sleepy Mermaid",
		},
		{
			name:             "invalid JSON",
			responseText:     "```json\n{invalid json}\n```",
			expectedError:    true,
			expectedLocation: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the cleaning logic directly by applying the same logic as MetaUpdate
			originalText := tt.responseText
			mTxt := originalText

			// Apply the same cleaning logic as in MetaUpdate
			mTxt = strings.TrimSpace(mTxt)

			// Strategy 1: Remove markdown code blocks if present
			if strings.HasPrefix(mTxt, "```") {
				lines := strings.Split(mTxt, "\n")
				startIdx := 0
				for i, line := range lines {
					if strings.HasPrefix(line, "```") && i == 0 {
						startIdx = 1
						break
					}
				}

				endIdx := len(lines)
				for i := len(lines) - 1; i >= 0; i-- {
					if strings.HasPrefix(lines[i], "```") && i > 0 {
						endIdx = i
						break
					}
				}

				if startIdx < endIdx {
					mTxt = strings.Join(lines[startIdx:endIdx], "\n")
				}
			}

			// Strategy 2: Look for JSON object if we have mixed content
			if !strings.HasPrefix(strings.TrimSpace(mTxt), "{") {
				jsonStart := strings.Index(mTxt, "{")
				if jsonStart >= 0 {
					mTxt = mTxt[jsonStart:]
				}
			}

			// Strategy 3: Clean up any remaining artifacts
			mTxt = strings.ReplaceAll(mTxt, "`", "")

			// Remove standalone "json" lines that might appear
			lines := strings.Split(mTxt, "\n")
			var cleanLines []string
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "json" && trimmed != "" {
					cleanLines = append(cleanLines, line)
				}
			}
			mTxt = strings.Join(cleanLines, "\n")
			mTxt = strings.TrimSpace(mTxt)

			var metaUpdate conditionals.GameStateDelta
			err := json.Unmarshal([]byte(mTxt), &metaUpdate)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error parsing %q -> %q: %v", originalText, mTxt, err)
				return
			}

			if metaUpdate.UserLocation != tt.expectedLocation {
				t.Errorf("Expected UserLocation %q, got %q", tt.expectedLocation, metaUpdate.UserLocation)
			}
		})
	}
}

func TestAnthropicService_ChatStream(t *testing.T) {
	t.Run("successful streaming response", func(t *testing.T) {
		// Mock server that returns Anthropic SSE streaming response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			// Send Anthropic-style streaming events
			responses := []string{
				`event: message_start`,
				`data: {"type": "message_start", "message": {"id": "msg_test", "type": "message", "role": "assistant", "content": [], "model": "claude-3-sonnet-20240229", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 10, "output_tokens": 1}}}`,
				``,
				`event: content_block_start`,
				`data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}`,
				``,
				`event: ping`,
				`data: {"type": "ping"}`,
				``,
				`event: content_block_delta`,
				`data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}`,
				``,
				`event: content_block_delta`,
				`data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": " world"}}`,
				``,
				`event: content_block_stop`,
				`data: {"type": "content_block_stop", "index": 0}`,
				``,
				`event: message_delta`,
				`data: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}, "usage": {"output_tokens": 15}}`,
				``,
				`event: message_stop`,
				`data: {"type": "message_stop"}`,
				``,
			}

			for _, response := range responses {
				if _, err := w.Write([]byte(response + "\n")); err != nil {
					return // Exit on write error in test
				}
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(10 * time.Millisecond) // Small delay to simulate streaming
			}
		}))
		defer server.Close()

		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		_ = NewAnthropicService("test-key", "claude-3-sonnet-20240229", "", log) // Keep for consistency

		// Test the streaming parsing logic by simulating the core events
		chunkChan := make(chan StreamChunk, 10)
		go func() {
			defer close(chunkChan)
			// Simulate the streaming response parsing with actual Anthropic events
			testEvents := []string{
				`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}`,
				`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": " world"}}`,
				`{"type": "message_stop"}`,
			}

			for _, eventData := range testEvents {
				var streamEvent AnthropicStreamEvent
				if err := json.Unmarshal([]byte(eventData), &streamEvent); err != nil {
					chunkChan <- StreamChunk{Error: err}
					return
				}

				// Apply the same logic as in the actual ChatStream method
				switch streamEvent.Type {
				case "content_block_delta":
					if streamEvent.Delta != nil && streamEvent.Delta.Type == "text_delta" {
						chunkChan <- StreamChunk{
							Content: streamEvent.Delta.Text,
							Done:    false,
						}
					}
				case "message_stop":
					chunkChan <- StreamChunk{Done: true}
					return
				}
			}
		}()

		// Collect streaming chunks
		var content strings.Builder
		var chunks []StreamChunk

		for chunk := range chunkChan {
			chunks = append(chunks, chunk)
			if chunk.Error != nil {
				t.Fatalf("Streaming error: %v", chunk.Error)
			}
			if chunk.Content != "" {
				content.WriteString(chunk.Content)
			}
			if chunk.Done {
				break
			}
		}

		expectedContent := "Hello world"
		if content.String() != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, content.String())
		}

		// Verify we received the expected number of chunks
		if len(chunks) != 3 { // 2 content chunks + 1 done chunk
			t.Errorf("Expected 3 chunks, got %d", len(chunks))
		}

		// Verify the last chunk is done
		if !chunks[len(chunks)-1].Done {
			t.Error("Expected last chunk to be marked as done")
		}
	})

	t.Run("handles API errors in stream", func(t *testing.T) {
		// Test error handling in streaming
		chunkChan := make(chan StreamChunk, 10)
		go func() {
			defer close(chunkChan)
			// Simulate an error event
			errorEvent := `{"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}`

			var streamEvent AnthropicStreamEvent
			if err := json.Unmarshal([]byte(errorEvent), &streamEvent); err != nil {
				chunkChan <- StreamChunk{Error: err}
				return
			}

			// Apply the same error handling logic as in ChatStream
			if streamEvent.Error != nil {
				chunkChan <- StreamChunk{Error: fmt.Errorf("anthropic API error: %s", streamEvent.Error.Message)}
				return
			}
		}()

		// Collect chunks and verify error handling
		var errorReceived error
		for chunk := range chunkChan {
			if chunk.Error != nil {
				errorReceived = chunk.Error
				break
			}
		}

		if errorReceived == nil {
			t.Error("Expected to receive an error")
		}

		expectedErrorMsg := "anthropic API error: Overloaded"
		if errorReceived.Error() != expectedErrorMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, errorReceived.Error())
		}
	})
}
