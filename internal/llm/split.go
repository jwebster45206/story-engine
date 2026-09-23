package llm

import (
	"context"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// splitService routes narrator streaming to one vendor and referee/reducer
// calls to another. The game still selects a single provider name.
type splitService struct {
	narrator LLMService
	backend  LLMService
}

func (s *splitService) ChatStream(ctx context.Context, messages []chat.ChatMessage, temperature float64) (<-chan StreamChunk, error) {
	return s.narrator.ChatStream(ctx, messages, temperature)
}

func (s *splitService) GetRuling(ctx context.Context, messages []chat.ChatMessage) (*chat.Ruling, Usage, error) {
	return s.backend.GetRuling(ctx, messages)
}

func (s *splitService) DeltaUpdate(ctx context.Context, messages []chat.ChatMessage) (*conditionals.GameStateDelta, Usage, error) {
	return s.backend.DeltaUpdate(ctx, messages)
}
