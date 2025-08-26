package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
)

func TestNewAnthropicService(t *testing.T) {
	apiKey := "test-api-key"
	modelName := "claude-3-sonnet-20240229"

	service := NewAnthropicService(apiKey, modelName)

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
	service := NewAnthropicService("test-key", "claude-3-sonnet-20240229")

	err := service.InitModel(context.Background(), "test-model")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestAnthropicService_ExtractSystemMessage(t *testing.T) {
	service := NewAnthropicService("test-key", "claude-3-sonnet-20240229")

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
			systemPrompt, nonSystemMessages := service.extractSystemMessage(tt.messages)

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
	req := AnthropicChatRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   1024,
		Temperature: 0.7,
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
