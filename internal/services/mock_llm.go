package services

import (
	"context"

	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

// MockLLMService is a mock implementation of LLMService for testing
type MockLLMService struct {
	InitModelFunc        func(ctx context.Context, modelName string) error
	GenerateResponseFunc func(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)
	IsModelReadyFunc     func(ctx context.Context, modelName string) (bool, error)

	// Track calls for testing
	InitModelCalls        []string
	GenerateResponseCalls []GenerateResponseCall
	IsModelReadyCalls     []string
}

type GenerateResponseCall struct {
	Messages []chat.ChatMessage
}

// NewMockLLMService creates a new mock LLM service
func NewMockLLMService() *MockLLMService {
	return &MockLLMService{
		InitModelCalls:        make([]string, 0),
		GenerateResponseCalls: make([]GenerateResponseCall, 0),
		IsModelReadyCalls:     make([]string, 0),
	}
}

// InitModel mocks model initialization
func (m *MockLLMService) InitModel(ctx context.Context, modelName string) error {
	m.InitModelCalls = append(m.InitModelCalls, modelName)

	if m.InitModelFunc != nil {
		return m.InitModelFunc(ctx, modelName)
	}

	// Default behavior - success
	return nil
}

// GenerateResponse mocks response generation
func (m *MockLLMService) GenerateResponse(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	m.GenerateResponseCalls = append(m.GenerateResponseCalls, GenerateResponseCall{
		Messages: messages,
	})

	if m.GenerateResponseFunc != nil {
		return m.GenerateResponseFunc(ctx, messages)
	}

	// Default behavior - return a mock response
	return &chat.ChatResponse{
		Message: "Mock response",
	}, nil
}

// IsModelReady mocks model readiness check
func (m *MockLLMService) IsModelReady(ctx context.Context, modelName string) (bool, error) {
	m.IsModelReadyCalls = append(m.IsModelReadyCalls, modelName)

	if m.IsModelReadyFunc != nil {
		return m.IsModelReadyFunc(ctx, modelName)
	}

	// Default behavior - model is ready
	return true, nil
}

// Reset clears all call tracking
func (m *MockLLMService) Reset() {
	m.InitModelCalls = make([]string, 0)
	m.GenerateResponseCalls = make([]GenerateResponseCall, 0)
	m.IsModelReadyCalls = make([]string, 0)
}

// SetInitModelError sets up the mock to return an error on InitModel
func (m *MockLLMService) SetInitModelError(err error) {
	m.InitModelFunc = func(ctx context.Context, modelName string) error {
		return err
	}
}

// SetGenerateResponseError sets up the mock to return an error on GenerateResponse
func (m *MockLLMService) SetGenerateResponseError(err error) {
	m.GenerateResponseFunc = func(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
		return nil, err
	}
}

// SetIsModelReadyError sets up the mock to return an error on IsModelReady
func (m *MockLLMService) SetIsModelReadyError(err error) {
	m.IsModelReadyFunc = func(ctx context.Context, modelName string) (bool, error) {
		return false, err
	}
}

// SetModelNotReady sets up the mock to return false for IsModelReady
func (m *MockLLMService) SetModelNotReady() {
	m.IsModelReadyFunc = func(ctx context.Context, modelName string) (bool, error) {
		return false, nil
	}
}
