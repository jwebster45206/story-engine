package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/jwebster45206/roleplay-agent/pkg/chat"
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

	response, err := mockService.GetChatResponse(context.Background(), messages)
	if err != nil {
		t.Errorf("GenerateResponse failed: %v", err)
	}

	if response.Message != "Mock response" {
		t.Errorf("Expected 'Mock response', got '%s'", response.Message)
	}

	if len(mockService.GenerateResponseCalls) != 1 {
		t.Errorf("Expected 1 GenerateResponse call, got %d", len(mockService.GenerateResponseCalls))
	}

	// Test IsModelReady
	ready, err := mockService.IsModelReady(context.Background(), "test-model")
	if err != nil {
		t.Errorf("IsModelReady failed: %v", err)
	}

	if !ready {
		t.Errorf("Expected model to be ready")
	}

	if len(mockService.IsModelReadyCalls) != 1 {
		t.Errorf("Expected 1 IsModelReady call, got %d", len(mockService.IsModelReadyCalls))
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

	// Test model not ready
	mockService.SetModelNotReady()
	ready, err := mockService.IsModelReady(context.Background(), "test-model")
	if err != nil {
		t.Errorf("IsModelReady failed: %v", err)
	}

	if ready {
		t.Errorf("Expected model to not be ready")
	}
}
