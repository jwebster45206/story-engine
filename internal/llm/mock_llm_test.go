package llm

import (
	"context"
	"fmt"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
)

func TestMockLLMService_ChatStream(t *testing.T) {
	mockService := NewMockLLMAPI()

	messages := []chat.ChatMessage{
		{Role: chat.ChatRoleUser, Content: "Hello"},
	}

	ch, err := mockService.ChatStream(context.Background(), messages, DefaultTemperature)
	if err != nil {
		t.Errorf("ChatStream failed: %v", err)
	}

	var content string
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		content += chunk.Content
	}
	if content != "Mock response" {
		t.Errorf("Expected 'Mock response', got '%s'", content)
	}

	generateCalls := mockService.GetCalls()
	if len(generateCalls) != 1 {
		t.Errorf("Expected 1 ChatStream call, got %d", len(generateCalls))
	}
}

func TestMockLLMService_ChatStreamError(t *testing.T) {
	mockService := NewMockLLMAPI()

	expectedErr := fmt.Errorf("generation failed")
	mockService.SetGenerateResponseError(expectedErr)

	_, err := mockService.ChatStream(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "x"}}, DefaultTemperature)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error '%s', got '%s'", expectedErr.Error(), err.Error())
	}
}
