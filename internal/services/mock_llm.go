package services

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/prompts"
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

// DeltaUpdate mocks the DeltaUpdate functionality
func (m *MockLLMAPI) DeltaUpdate(ctx context.Context, messages []chat.ChatMessage) (*state.GameStateDelta, string, error) {
	// For testing, return a simple mock DeltaUpdate
	t := true
	f := false
	return &state.GameStateDelta{
		UserLocation: "mock_location",
		SceneChange: &struct {
			To     string `json:"to"`
			Reason string `json:"reason"`
		}{
			To:     "mock_scene",
			Reason: "testing",
		},
		ItemEvents: []struct {
			Item   string `json:"item"`
			Action string `json:"action"` // enum
			From   *struct {
				Type string `json:"type"`
				Name string `json:"name,omitempty"`
			} `json:"from,omitempty"`
			To *struct {
				Type string `json:"type"`
				Name string `json:"name,omitempty"`
			} `json:"to,omitempty"`
			Consumed *bool `json:"consumed,omitempty"`
		}{
			{
				Item:   "mock_item",
				Action: "add",
				From: &struct {
					Type string `json:"type"`
					Name string `json:"name,omitempty"`
				}{
					Type: "inventory",
					Name: "user_inventory",
				},
				To: &struct {
					Type string `json:"type"`
					Name string `json:"name,omitempty"`
				}{
					Type: "inventory",
					Name: "user_inventory",
				},
				Consumed: &t,
			},
		},
		SetVars: map[string]string{
			"mock_var": "mock_value",
		},
		GameEnded: &f,
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
		promptPrefix := prompts.ReducerPrompt
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

// ChatStream mocks streaming response generation
func (m *MockLLMAPI) ChatStream(ctx context.Context, messages []chat.ChatMessage) (<-chan StreamChunk, error) {
	return nil, fmt.Errorf("streaming not implemented for mock LLM")
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
