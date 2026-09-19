package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

const (
	anthropicBaseURL      = "https://api.anthropic.com/v1"
	anthropicVersion      = "2023-06-01"
	anthropicCacheTTLBeta = "extended-cache-ttl-2025-04-11"
	vendorAnthropic       = "anthropic"
	anthropicCacheTTLHour = "1h"
)

// AnthropicService implements LLMService for Anthropic Claude
type AnthropicService struct {
	apiKey           string
	baseURL          string
	modelName        string
	backendModelName string
	httpClient       *http.Client
	logger           *slog.Logger
}

type AnthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type AnthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type AnthropicCacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

type AnthropicSystemBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
}

type AnthropicChatRequest struct {
	Model         string                 `json:"model"`
	MaxTokens     int                    `json:"max_tokens"`
	Messages      []chat.LLMMessage      `json:"messages"`
	System        []AnthropicSystemBlock `json:"system,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	StopSequences []string               `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool        `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice   `json:"tool_choice,omitempty"`
}

type AnthropicContentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type AnthropicStreamEvent struct {
	Type string `json:"type"`
	// For message_start, content_block_start, etc.
	Message      *AnthropicChatResponse `json:"message,omitempty"`
	ContentBlock *AnthropicContentBlock `json:"content_block,omitempty"`
	Index        *int                   `json:"index,omitempty"`
	// For content_block_delta
	Delta *AnthropicStreamDelta `json:"delta,omitempty"`
	// For message_delta
	Usage *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
	// For errors
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type AnthropicStreamDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type AnthropicChatResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewAnthropicService(pc *config.ProviderConfig, logger *slog.Logger) *AnthropicService {
	if logger != nil {
		logger.Debug("Anthropic sampling parameters (temperature, top_p, top_k) are not sent; Opus 4.7+ rejects non-default values")
	}
	return &AnthropicService{
		apiKey:           pc.APIKey,
		baseURL:          anthropicBaseURL,
		modelName:        pc.Model,
		backendModelName: pc.BackendModel,
		httpClient: &http.Client{
			Timeout: HTTPClientTimeout,
		},
		logger: logger,
	}
}

// splitChatMessages extracts system messages as ordered Anthropic content blocks.
// Persistent blocks are tagged for prompt cache. Trailing system messages
// (e.g. the game-end prompt) are hoisted with the rest of the system prefix.
func (a *AnthropicService) splitChatMessages(messages []chat.ChatMessage) ([]AnthropicSystemBlock, []chat.ChatMessage) {
	var systemBlocks []AnthropicSystemBlock
	var nonSystemMessages []chat.ChatMessage

	for _, msg := range messages {
		if msg.Role != chat.ChatRoleSystem {
			nonSystemMessages = append(nonSystemMessages, msg)
			continue
		}
		block := AnthropicSystemBlock{
			Type: "text",
			Text: msg.Content,
		}
		if msg.IsPersistent {
			block.CacheControl = &AnthropicCacheControl{
				Type: "ephemeral",
				TTL:  anthropicCacheTTLHour,
			}
		}
		systemBlocks = append(systemBlocks, block)
	}
	return systemBlocks, nonSystemMessages
}

func (a *AnthropicService) setAnthropicHeaders(req *http.Request) {
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("anthropic-beta", anthropicCacheTTLBeta)
	req.Header.Set("content-type", "application/json")
}

// chatCompletion is the non-streaming backend path. BackendMaxTokens is the cap by convention.
// Sampling params are not sent (deprecated on Opus 4.7+).
func (a *AnthropicService) chatCompletion(ctx context.Context, messages []chat.ChatMessage, modelName string, tools []AnthropicTool) (string, Usage, error) {
	// Extract system messages and convert to Anthropic format
	systemPrompt, conversationMessages := a.splitChatMessages(messages)

	anthropicReq := AnthropicChatRequest{
		Model:     modelName,
		MaxTokens: BackendMaxTokens,
		Messages:  chat.ToLLMMessages(conversationMessages),
		Stream:    false,
	}

	// Add system prompt if we have one
	if len(systemPrompt) > 0 {
		anthropicReq.System = systemPrompt
	}

	// Add tools if provided, and use the first tool as the tool choice
	if len(tools) > 0 {
		anthropicReq.Tools = tools
		anthropicReq.ToolChoice = &AnthropicToolChoice{
			Type: "tool",
			Name: tools[0].Name,
		}
	}

	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to create request: %w", err)
	}

	a.setAnthropicHeaders(req)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", Usage{}, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp AnthropicChatResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return "", Usage{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		return "", Usage{}, fmt.Errorf("API error: %s", anthropicResp.Error.Message)
	}

	usage := Usage{
		InputTokens:              anthropicResp.Usage.InputTokens,
		OutputTokens:             anthropicResp.Usage.OutputTokens,
		CacheCreationInputTokens: anthropicResp.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     anthropicResp.Usage.CacheReadInputTokens,
		Model:                    modelName,
		Vendor:                   vendorAnthropic,
	}
	if anthropicResp.Model != "" {
		usage.Model = anthropicResp.Model
	}

	// Extract content from the response (text or tool use)
	var responseText string
	for _, content := range anthropicResp.Content {
		switch content.Type {
		case "text":
			responseText += content.Text
		case "tool_use":
			// For tool use, return the input as JSON
			inputBytes, err := json.Marshal(content.Input)
			if err != nil {
				return "", usage, fmt.Errorf("failed to marshal tool input: %w", err)
			}
			responseText += string(inputBytes)
		}
	}

	if responseText == "" {
		responseText = "(no response)"
	}

	return responseText, usage, nil
}

// ChatStream generates a streaming chat response using Anthropic.
func (a *AnthropicService) ChatStream(ctx context.Context, messages []chat.ChatMessage, temperature float64) (<-chan StreamChunk, error) {
	_ = temperature // retained for LLMService interface; not sent to Anthropic
	// Extract system messages and convert to Anthropic format
	systemPrompt, conversationMessages := a.splitChatMessages(messages)

	anthropicReq := AnthropicChatRequest{
		Model:     a.modelName,
		MaxTokens: DefaultMaxTokens,
		Messages:  chat.ToLLMMessages(conversationMessages),
		Stream:    true,
	}

	// Add system prompt if we have one
	if len(systemPrompt) > 0 {
		anthropicReq.System = systemPrompt
	}

	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	a.setAnthropicHeaders(req)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("API request failed with status %d (also failed to read body: %w)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	chunkChan := make(chan StreamChunk, 10)

	go func() {
		defer func() { _ = resp.Body.Close() }()
		defer close(chunkChan)

		scanner := bufio.NewScanner(resp.Body)
		usage := Usage{Model: a.modelName, Vendor: vendorAnthropic}
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				chunkChan <- StreamChunk{Error: ctx.Err(), Usage: usage}
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			// Anthropic streaming responses are in SSE format
			// Lines can be "event: <event_type>" or "data: <json>"
			if strings.HasPrefix(line, "event: ") {
				// Event type line, we can ignore these as we parse by data content
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			// Remove "data: " prefix
			jsonData := strings.TrimPrefix(line, "data: ")

			var streamEvent AnthropicStreamEvent
			if err := json.Unmarshal([]byte(jsonData), &streamEvent); err != nil {
				chunkChan <- StreamChunk{Error: fmt.Errorf("failed to decode streaming response: %w", err), Usage: usage}
				return
			}

			// Check for API errors
			if streamEvent.Error != nil {
				chunkChan <- StreamChunk{Error: fmt.Errorf("anthropic API error: %s", streamEvent.Error.Message), Usage: usage}
				return
			}

			// Handle different event types
			switch streamEvent.Type {
			case "content_block_delta":
				// This contains the actual text content
				if streamEvent.Delta != nil && streamEvent.Delta.Type == "text_delta" {
					chunkChan <- StreamChunk{
						Content: streamEvent.Delta.Text,
						Done:    false,
					}
				}
			case "message_start":
				if streamEvent.Message != nil {
					usage.InputTokens = streamEvent.Message.Usage.InputTokens
					usage.CacheCreationInputTokens = streamEvent.Message.Usage.CacheCreationInputTokens
					usage.CacheReadInputTokens = streamEvent.Message.Usage.CacheReadInputTokens
					if streamEvent.Message.Usage.OutputTokens > 0 {
						usage.OutputTokens = streamEvent.Message.Usage.OutputTokens
					}
					if streamEvent.Message.Model != "" {
						usage.Model = streamEvent.Message.Model
					}
				}
			case "message_delta":
				if streamEvent.Usage != nil {
					usage.OutputTokens = streamEvent.Usage.OutputTokens
				}
			case "message_stop":
				chunkChan <- StreamChunk{Done: true, Usage: usage}
				return
			case "content_block_start", "content_block_stop", "ping":
				continue
			default:
				// Unknown event type, log and continue
				a.logger.Debug("Unknown Anthropic stream event type", "type", streamEvent.Type)
				continue
			}
		}

		if err := scanner.Err(); err != nil {
			chunkChan <- StreamChunk{Error: fmt.Errorf("error reading stream: %w", err), Usage: usage}
		}
	}()

	return chunkChan, nil
}

// getDeltaUpdateTool returns the tool definition for gamestate deltas
func (a *AnthropicService) getDeltaUpdateTool() AnthropicTool {
	return AnthropicTool{
		Name:        "apply_changes",
		Description: "Return only the delta for game state updates.",
		InputSchema: deltaUpdateSchema(),
	}
}

// DeltaUpdate processes a gamestate delta request using Anthropic Claude
func (a *AnthropicService) DeltaUpdate(ctx context.Context, messages []chat.ChatMessage) (*conditionals.GameStateDelta, Usage, error) {
	// Determine which model to use for DeltaUpdate
	modelToUse := a.modelName
	if a.backendModelName != "" {
		modelToUse = a.backendModelName
	}

	// Create tools for structured output (first tool will be automatically chosen)
	tools := []AnthropicTool{a.getDeltaUpdateTool()}

	content, usage, err := a.chatCompletion(ctx, messages, modelToUse, tools)
	if err != nil {
		return nil, usage, err
	}

	deltaUpdate, repaired, err := parseDeltaUpdateResponse(content)
	if err != nil {
		return nil, usage, err
	}
	if repaired {
		a.logger.Warn("fixed truncated gamestate delta JSON",
			"backend_model", modelToUse,
			"original_len", len(content),
			"preview", content,
		)
	}

	return deltaUpdate, usage, nil
}

func (a *AnthropicService) getRulingTool() AnthropicTool {
	return AnthropicTool{
		Name:        "ruling",
		Description: "Return the referee ruling for the player's action.",
		InputSchema: rulingSchema(),
	}
}

// GetRuling generates a structured referee ruling on the backend model.
func (a *AnthropicService) GetRuling(ctx context.Context, messages []chat.ChatMessage) (*chat.Ruling, Usage, error) {
	modelToUse := a.modelName
	if a.backendModelName != "" {
		modelToUse = a.backendModelName
	}

	tools := []AnthropicTool{a.getRulingTool()}
	content, usage, err := a.chatCompletion(ctx, messages, modelToUse, tools)
	if err != nil {
		return nil, usage, err
	}

	ruling, err := parseRulingResponse(content)
	if err != nil {
		return nil, usage, err
	}
	return ruling, usage, nil
}
