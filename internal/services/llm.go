package services

import (
	"context"

	"github.com/jwebster45206/story-engine/pkg/chat"
)

// LLMService defines the interface for interacting with the LLM API
type LLMService interface {
	InitModel(ctx context.Context, modelName string) error

	// Chat generates a chat response using the LLM
	Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)
	MetaUpdate(ctx context.Context, messages []chat.ChatMessage) (*chat.MetaUpdate, error)
}
