package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
)

func TestMockLLMService(t *testing.T) {
	mockService := NewMockLLMAPI()

	// Test InitializeModel
	err := mockService.InitModel(context.Background(), "test-model")
	if err != nil {
		t.Errorf("InitializeModel failed: %v", err)
	}

	if len(mockService.InitModelCalls) != 1 {
		t.Errorf("Expected 1 InitializeModel call, got %d", len(mockService.InitModelCalls))
	}

	if mockService.InitModelCalls[0] != "test-model" {
		t.Errorf("Expected model name 'test-model', got '%s'", mockService.InitModelCalls[0])
	}

	// Test GenerateResponse
	messages := []chat.ChatMessage{
		{Role: chat.ChatRoleUser, Content: "Hello"},
	}

	response, err := mockService.Chat(context.Background(), messages)
	if err != nil {
		t.Errorf("GenerateResponse failed: %v", err)
	}

	if response.Message != "Mock response" {
		t.Errorf("Expected 'Mock response', got '%s'", response.Message)
	}

	_, generateCalls := mockService.GetCalls()
	if len(generateCalls) != 1 {
		t.Errorf("Expected 1 GenerateResponse call, got %d", len(generateCalls))
	}
}

func TestMockLLMService_ErrorHandling(t *testing.T) {
	mockService := NewMockLLMAPI()

	// Test InitializeModel error
	expectedErr := fmt.Errorf("initialization failed")
	mockService.SetInitModelError(expectedErr)

	err := mockService.InitModel(context.Background(), "test-model")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error '%s', got '%s'", expectedErr.Error(), err.Error())
	}
}
