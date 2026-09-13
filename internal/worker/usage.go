package worker

import (
	"log/slog"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/llm"
)

const (
	usageTypeNarrator    = "narrator"
	usageTypeReducer     = "reducer"
	usageTypeAdjudicator = "adjudicator"
)

func logLLMUsage(logger *slog.Logger, typ string, gameStateID, principalID uuid.UUID, provider string, usage llm.Usage) {
	if logger == nil {
		return
	}
	if usage.InputTokens > 0 {
		logger.Info("llm usage",
			"type", typ,
			"game_state_id", gameStateID,
			"principal_id", principalID,
			"usage", usage,
		)
		return
	}
	logger.Warn("llm usage missing",
		"type", typ,
		"game_state_id", gameStateID,
		"principal_id", principalID,
		"provider", provider,
	)
}
