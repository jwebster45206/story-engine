package services

import (
	"context"

	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

// LLMService defines the interface for interacting with the LLM API
type LLMService interface {
	// InitializeModel initializes the LLM model on startup
	InitializeModel(ctx context.Context, modelName string) error

	// GenerateResponse generates a chat response using the LLM
	GenerateResponse(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)

	// IsModelReady checks if the specified model is ready for use
	IsModelReady(ctx context.Context, modelName string) (bool, error)
}
