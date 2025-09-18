package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const (
	DefaultTemperature = 0.7
	DefaultMaxTokens   = 512
	BackendMaxTokens   = 512
)

// StreamChunk represents a chunk of streaming response
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   error  `json:"error,omitempty"`
}

// LLMService defines the interface for interacting with the LLM API
type LLMService interface {
	InitModel(ctx context.Context, modelName string) error

	// Chat generates a chat response using the LLM
	Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error)
	
	// ChatStream generates a streaming chat response using the LLM
	ChatStream(ctx context.Context, messages []chat.ChatMessage) (<-chan StreamChunk, error)
	
	DeltaUpdate(ctx context.Context, messages []chat.ChatMessage) (*state.GameStateDelta, string, error)
}

// parseDeltaUpdateResponse parses an LLM response text into a DeltaUpdate struct.
// It handles various response formats including markdown code blocks, mixed content,
// and other common artifacts that LLMs might include in their JSON responses.
func parseDeltaUpdateResponse(responseText string) (*state.GameStateDelta, error) {
	if responseText == "" {
		return nil, nil
	}

	originalText := responseText
	mTxt := strings.TrimSpace(originalText)

	// Remove markdown code blocks if present
	if strings.HasPrefix(mTxt, "```") {
		lines := strings.Split(mTxt, "\n")
		startIdx := 0
		for i, line := range lines {
			if strings.HasPrefix(line, "```") && i == 0 {
				startIdx = 1
				break
			}
		}
		endIdx := len(lines)
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.HasPrefix(lines[i], "```") && i > 0 {
				endIdx = i
				break
			}
		}
		if startIdx < endIdx {
			mTxt = strings.Join(lines[startIdx:endIdx], "\n")
		}
	}

	// Look for JSON object if we have mixed content
	if !strings.HasPrefix(strings.TrimSpace(mTxt), "{") {
		jsonStart := strings.Index(mTxt, "{")
		if jsonStart >= 0 {
			mTxt = mTxt[jsonStart:]
		}
	}

	// Clean up any remaining artifacts
	mTxt = strings.ReplaceAll(mTxt, "`", "")

	// Remove standalone "json" lines that might appear
	lines := strings.Split(mTxt, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "json" && trimmed != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	mTxt = strings.Join(cleanLines, "\n")
	mTxt = strings.TrimSpace(mTxt)

	var metaUpdate state.GameStateDelta
	if err := json.Unmarshal([]byte(mTxt), &metaUpdate); err != nil {
		return nil, fmt.Errorf("failed to parse gamestate delta. Original response: %q, Cleaned text: %q, Error: %w", originalText, mTxt, err)
	}

	return &metaUpdate, nil
}
