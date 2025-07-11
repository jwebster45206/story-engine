package services

import (
	"context"

	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

// LLMService defines the interface for interacting with the LLM API
type LLMService interface {
	InitModel(ctx context.Context, modelName string) error
	IsModelReady(ctx context.Context, modelName string) (bool, error)

	// GenerateResponse generates a chat response using the LLM
	GenerateResponse(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)
}
