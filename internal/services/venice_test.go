package services

import (
	"context"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
)

func TestNewVeniceService(t *testing.T) {
	apiKey := "test-api-key"
	modelName := "test-model"

	service := NewVeniceService(apiKey, modelName)

	if service.apiKey != apiKey {
		t.Errorf("Expected apiKey %s, got %s", apiKey, service.apiKey)
	}

	if service.modelName != modelName {
		t.Errorf("Expected modelName %s, got %s", modelName, service.modelName)
	}

	if service.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}
}

func TestVeniceService_InitModel(t *testing.T) {
	service := NewVeniceService("invalid-key", "test-model")

	// This should not fail even with invalid key since we handle the error gracefully
	err := service.InitModel(context.Background(), "test-model")
	// We expect this to fail with invalid key, but it should not panic
	if err == nil {
		t.Log("InitModel succeeded (possibly due to graceful error handling)")
	} else {
		t.Logf("InitModel failed as expected with invalid key: %v", err)
	}
}

// Mock test for chat response structure
func TestVeniceChatRequestStructure(t *testing.T) {
	messages := []chat.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	req := VeniceChatRequest{
		Model:       "test-model",
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   1000,
		Stream:      false,
	}

	if req.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", req.Model)
	}

	if len(req.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(req.Messages))
	}

	if req.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", req.Temperature)
	}
}
