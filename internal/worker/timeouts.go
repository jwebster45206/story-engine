package worker

import (
	"time"

	"github.com/jwebster45206/story-engine/internal/services"
)

const (
	// LLMRequestTimeout bounds a single LLM chat/stream call (matches HTTP client timeout).
	LLMRequestTimeout = services.HTTPClientTimeout

	// GameLockTTL is how long a worker holds the per-game Redis lock for stream + save.
	// Longer than LLMRequestTimeout so the lock cannot expire mid-stream.
	GameLockTTL = LLMRequestTimeout + 30*time.Second

	// DeltaTimeout bounds background gamestate delta (reducer) extraction.
	DeltaTimeout = LLMRequestTimeout
)
