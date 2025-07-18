package services

import (
	"context"
	"strings"
	"sync"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// MockLLMAPI is a mock implementation of LLMService for testing
type MockLLMAPI struct {
	InitModelFunc        func(ctx context.Context, modelName string) error
	GenerateResponseFunc func(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)
	IsModelReadyFunc     func(ctx context.Context, modelName string) (bool, error)
	ListModelsFunc       func(ctx context.Context) ([]string, error)

	// Track calls for testing
	InitModelCalls        []string
	GenerateResponseCalls []GenerateResponseCall
	IsModelReadyCalls     []string
	ListModelsCalls       []bool // Track calls to ListModels

	mu sync.Mutex // protects all fields above
}

type GenerateResponseCall struct {
	Messages []chat.ChatMessage
}

// NewMockLLMAPI creates a new mock LLM service
func NewMockLLMAPI() *MockLLMAPI {
	return &MockLLMAPI{
		InitModelCalls:        make([]string, 0),
		GenerateResponseCalls: make([]GenerateResponseCall, 0),
		IsModelReadyCalls:     make([]string, 0),
		ListModelsCalls:       make([]bool, 0),
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

// GetChatResponse mocks response generation
func (m *MockLLMAPI) GetChatResponse(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GenerateResponseCalls = append(m.GenerateResponseCalls, GenerateResponseCall{
		Messages: messages,
	})

	if m.GenerateResponseFunc != nil {
		return m.GenerateResponseFunc(ctx, messages)
	}

	// Detect if this is a PromptState extraction request (meta update)
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

// ListModels mocks model listing
func (m *MockLLMAPI) ListModels(ctx context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ListModelsCalls = append(m.ListModelsCalls, true)

	if m.ListModelsFunc != nil {
		return m.ListModelsFunc(ctx)
	}

	// Default behavior - return some mock models
	return []string{"foo"}, nil
}

// IsModelReady mocks model readiness check
func (m *MockLLMAPI) IsModelReady(ctx context.Context, modelName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IsModelReadyCalls = append(m.IsModelReadyCalls, modelName)

	if m.IsModelReadyFunc != nil {
		return m.IsModelReadyFunc(ctx, modelName)
	}

	// Default behavior - model is ready
	return true, nil
}

// Reset clears all call tracking
func (m *MockLLMAPI) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InitModelCalls = make([]string, 0)
	m.GenerateResponseCalls = make([]GenerateResponseCall, 0)
	m.IsModelReadyCalls = make([]string, 0)
	m.ListModelsCalls = make([]bool, 0)
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

// SetIsModelReadyError sets up the mock to return an error on IsModelReady
func (m *MockLLMAPI) SetIsModelReadyError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IsModelReadyFunc = func(ctx context.Context, modelName string) (bool, error) {
		return false, err
	}
}

// SetListModelsError sets up the mock to return an error on ListModels
func (m *MockLLMAPI) SetListModelsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListModelsFunc = func(ctx context.Context) ([]string, error) {
		return nil, err
	}
}

// SetListModelsResponse sets up the mock to return specific models
func (m *MockLLMAPI) SetListModelsResponse(models []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListModelsFunc = func(ctx context.Context) ([]string, error) {
		return models, nil
	}
}

// SetModelNotReady sets up the mock to return false for IsModelReady
func (m *MockLLMAPI) SetModelNotReady() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IsModelReadyFunc = func(ctx context.Context, modelName string) (bool, error) {
		return false, nil
	}
}

// GetCalls returns a copy of the call tracking data in a thread-safe way
func (m *MockLLMAPI) GetCalls() ([]string, []GenerateResponseCall, []string, []bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	initCalls := make([]string, len(m.InitModelCalls))
	copy(initCalls, m.InitModelCalls)

	respCalls := make([]GenerateResponseCall, len(m.GenerateResponseCalls))
	copy(respCalls, m.GenerateResponseCalls)

	readyCalls := make([]string, len(m.IsModelReadyCalls))
	copy(readyCalls, m.IsModelReadyCalls)

	listCalls := make([]bool, len(m.ListModelsCalls))
	copy(listCalls, m.ListModelsCalls)

	return initCalls, respCalls, readyCalls, listCalls
}
