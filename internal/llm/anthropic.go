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
	anthropicBaseURL = "https://api.anthropic.com/v1"
	anthropicVersion = "2023-06-01"
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

type AnthropicChatRequest struct {
	Model         string               `json:"model"`
	MaxTokens     int                  `json:"max_tokens"`
	Messages      []chat.LLMMessage    `json:"messages"`
	System        string               `json:"system,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool      `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice `json:"tool_choice,omitempty"`
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
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
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

// splitChatMessages extracts and combines all system messages into a single top-level
// Anthropic system prompt. Trailing system messages (e.g. the game-end prompt appended
// after the user turn) are hoisted to the front here; Venice leaves them inline.
func (a *AnthropicService) splitChatMessages(messages []chat.ChatMessage) (string, []chat.ChatMessage) {
	var systemParts []string
	var nonSystemMessages []chat.ChatMessage

	for _, msg := range messages {
		if msg.Role == chat.ChatRoleSystem {
			systemParts = append(systemParts, msg.Content)
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	systemPrompt := strings.Join(systemParts, "\n\n")
	return systemPrompt, nonSystemMessages
}

// chatCompletion makes a chat completion request to Anthropic with the specified model.
// temperature is used only to select DefaultMaxTokens vs BackendMaxTokens; it is never
// sent to the Anthropic API (sampling params are deprecated on Opus 4.7+).
func (a *AnthropicService) chatCompletion(ctx context.Context, messages []chat.ChatMessage, modelName string, temperature float64, tools []AnthropicTool) (string, error) {
	// Extract system messages and convert to Anthropic format
	systemPrompt, conversationMessages := a.splitChatMessages(messages)

	maxTokens := DefaultMaxTokens
	if temperature == 0 {
		maxTokens = BackendMaxTokens
	}
	anthropicReq := AnthropicChatRequest{
		Model:     modelName,
		MaxTokens: maxTokens,
		Messages:  chat.ToLLMMessages(conversationMessages),
		Stream:    false,
	}

	// Add system prompt if we have one
	if systemPrompt != "" {
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
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set required Anthropic headers
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp AnthropicChatResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		return "", fmt.Errorf("API error: %s", anthropicResp.Error.Message)
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
				return "", fmt.Errorf("failed to marshal tool input: %w", err)
			}
			responseText += string(inputBytes)
		}
	}

	if responseText == "" {
		responseText = "(no response)"
	}

	return responseText, nil
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
	if systemPrompt != "" {
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

	// Set required Anthropic headers
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

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
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				chunkChan <- StreamChunk{Error: ctx.Err()}
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
				chunkChan <- StreamChunk{Error: fmt.Errorf("failed to decode streaming response: %w", err)}
				return
			}

			// Check for API errors
			if streamEvent.Error != nil {
				chunkChan <- StreamChunk{Error: fmt.Errorf("anthropic API error: %s", streamEvent.Error.Message)}
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
			case "message_stop":
				// End of stream
				chunkChan <- StreamChunk{Done: true}
				return
			case "message_start", "content_block_start", "content_block_stop", "message_delta", "ping":
				// These are structural events we can ignore for our streaming purposes
				continue
			default:
				// Unknown event type, log and continue
				a.logger.Debug("Unknown Anthropic stream event type", "type", streamEvent.Type)
				continue
			}
		}

		if err := scanner.Err(); err != nil {
			chunkChan <- StreamChunk{Error: fmt.Errorf("error reading stream: %w", err)}
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
func (a *AnthropicService) DeltaUpdate(ctx context.Context, messages []chat.ChatMessage) (*conditionals.GameStateDelta, string, error) {
	// Determine which model to use for DeltaUpdate
	modelToUse := a.modelName
	if a.backendModelName != "" {
		modelToUse = a.backendModelName
	}

	// Create tools for structured output (first tool will be automatically chosen)
	tools := []AnthropicTool{a.getDeltaUpdateTool()}

	content, err := a.chatCompletion(ctx, messages, modelToUse, 0.0, tools)
	if err != nil {
		return nil, "", err
	}

	deltaUpdate, repaired, err := parseDeltaUpdateResponse(content)
	if err != nil {
		return nil, "", err
	}
	if repaired {
		a.logger.Warn("fixed truncated gamestate delta JSON",
			"backend_model", modelToUse,
			"original_len", len(content),
			"preview", content,
		)
	}

	return deltaUpdate, modelToUse, nil
}
