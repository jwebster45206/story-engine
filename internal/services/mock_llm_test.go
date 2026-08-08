package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
)

func TestMockLLMService(t *testing.T) {
	mockService := NewMockLLMAPI()

	messages := []chat.ChatMessage{
		{Role: chat.ChatRoleUser, Content: "Hello"},
	}

	response, err := mockService.Chat(context.Background(), messages, DefaultTemperature)
	if err != nil {
		t.Errorf("GenerateResponse failed: %v", err)
	}

	if response.Message != "Mock response" {
		t.Errorf("Expected 'Mock response', got '%s'", response.Message)
	}

	generateCalls := mockService.GetCalls()
	if len(generateCalls) != 1 {
		t.Errorf("Expected 1 GenerateResponse call, got %d", len(generateCalls))
	}
}

func TestMockLLMService_ErrorHandling(t *testing.T) {
	mockService := NewMockLLMAPI()

	expectedErr := fmt.Errorf("generation failed")
	mockService.SetGenerateResponseError(expectedErr)

	_, err := mockService.Chat(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "x"}}, DefaultTemperature)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error '%s', got '%s'", expectedErr.Error(), err.Error())
	}
}
