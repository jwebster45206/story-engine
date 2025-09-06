package services

import (
	"context"
	"strings"
	"sync"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// MockLLMAPI is a mock implementation of LLMService for testing
type MockLLMAPI struct {
	InitModelFunc        func(ctx context.Context, modelName string) error
	GenerateResponseFunc func(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)

	// Track calls for testing
	InitModelCalls        []string
	GenerateResponseCalls []GenerateResponseCall

	mu sync.Mutex // protects all fields above
}

// MetaUpdate mocks the MetaUpdate functionality
func (m *MockLLMAPI) MetaUpdate(ctx context.Context, messages []chat.ChatMessage) (*state.GameStateDelta, string, error) {
	// For testing, return a simple mock MetaUpdate
	return &state.GameStateDelta{
		UserLocation:        "mock_location",
		AddToInventory:      []string{"mock_item"},
		RemoveFromInventory: []string{"old_item"},
		MovedItems: []struct {
			Item string `json:"item"`
			From string `json:"from"`
			To   string `json:"to,omitempty"`
		}{
			{
				Item: "mock_item",
				From: "start",
				To:   "user_inventory",
			},
		},
		UpdatedNPCs: []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Location    string `json:"location"`
		}{
			{
				Name:        "Mock NPC",
				Description: "A mock NPC for testing.",
				Location:    "mock_location",
			},
		},
	}, "mock-model", nil
}

type GenerateResponseCall struct {
	Messages []chat.ChatMessage
}

// NewMockLLMAPI creates a new mock LLM service
func NewMockLLMAPI() *MockLLMAPI {
	return &MockLLMAPI{
		InitModelCalls:        make([]string, 0),
		GenerateResponseCalls: make([]GenerateResponseCall, 0),
	}
}

// InitModel mocks model initialization
func (m *MockLLMAPI) InitModel(ctx context.Context, modelName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.InitModelCalls = append(m.InitModelCalls, modelName)

	if m.InitModelFunc != nil {
		return m.InitModelFunc(ctx, modelName)
	}

	// Default behavior - success
	return nil
}

// Chat mocks response generation
func (m *MockLLMAPI) Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GenerateResponseCalls = append(m.GenerateResponseCalls, GenerateResponseCall{
		Messages: messages,
	})

	if m.GenerateResponseFunc != nil {
		return m.GenerateResponseFunc(ctx, messages)
	}

	// Detect if this is a PromptState extraction request (gamestate delta)
	if len(messages) > 0 && messages[0].Role == chat.ChatRoleSystem {
		promptPrefix := scenario.PromptStateExtractionInstructions
		if len(promptPrefix) > 50 {
			promptPrefix = promptPrefix[:50]
		}
		if strings.HasPrefix(messages[0].Content, promptPrefix) {
			return &chat.ChatResponse{
				Message: `{"location":"Test Location","flags":{"test_flag":true},"inventory":["test item"],"npcs":{"TestNPC":{"name":"TestNPC","type":"test","disposition":"neutral","description":"A test NPC.","important":true}}}`,
			}, nil
		}
	}

	return &chat.ChatResponse{
		Message: "Mock response",
	}, nil
}

// Reset clears all call tracking
func (m *MockLLMAPI) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InitModelCalls = make([]string, 0)
	m.GenerateResponseCalls = make([]GenerateResponseCall, 0)
}

// SetInitModelError sets up the mock to return an error on InitModel
func (m *MockLLMAPI) SetInitModelError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InitModelFunc = func(ctx context.Context, modelName string) error {
		return err
	}
}

// SetGenerateResponseError sets up the mock to return an error on GenerateResponse
func (m *MockLLMAPI) SetGenerateResponseError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GenerateResponseFunc = func(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
		return nil, err
	}
}

// GetCalls returns a copy of the call tracking data in a thread-safe way
func (m *MockLLMAPI) GetCalls() ([]string, []GenerateResponseCall) {
	m.mu.Lock()
	defer m.mu.Unlock()

	initCalls := make([]string, len(m.InitModelCalls))
	copy(initCalls, m.InitModelCalls)

	respCalls := make([]GenerateResponseCall, len(m.GenerateResponseCalls))
	copy(respCalls, m.GenerateResponseCalls)

	return initCalls, respCalls
}
