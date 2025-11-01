package services

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
	"time"

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
	Temperature   *float64             `json:"temperature,omitempty"`
	Messages      []chat.ChatMessage   `json:"messages"`
	System        string               `json:"system,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	TopK          *int                 `json:"top_k,omitempty"`
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

func NewAnthropicService(apiKey string, modelName string, backendModelName string, logger *slog.Logger) *AnthropicService {
	return &AnthropicService{
		apiKey:           apiKey,
		modelName:        modelName,
		backendModelName: backendModelName,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

func (a *AnthropicService) InitModel(ctx context.Context, modelName string) error {
	return nil
}

// splitChatMessages extracts and combines all system messages into a single system prompt
// and returns the remaining non-system messages
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

// Chat generates a chat response using Anthropic Claude
// chatCompletion makes a chat completion request to Anthropic with the specified model
func (a *AnthropicService) chatCompletion(ctx context.Context, messages []chat.ChatMessage, modelName string, temperature float64, tools []AnthropicTool) (string, error) {
	// Extract system messages and convert to Anthropic format
	systemPrompt, conversationMessages := a.splitChatMessages(messages)

	maxTokens := DefaultMaxTokens
	if temperature == 0 {
		maxTokens = BackendMaxTokens
	}
	anthropicReq := AnthropicChatRequest{
		Model:       modelName,
		MaxTokens:   maxTokens,
		Temperature: &temperature,
		Messages:    conversationMessages,
		Stream:      false,
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

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicBaseURL+"/messages", bytes.NewBuffer(reqBody))
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

func (a *AnthropicService) Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	content, err := a.chatCompletion(ctx, messages, a.modelName, DefaultTemperature, nil)
	if err != nil {
		return nil, err
	}

	return &chat.ChatResponse{
		Message: content,
	}, nil
}

// ChatStream generates a streaming chat response using Anthropic
func (a *AnthropicService) ChatStream(ctx context.Context, messages []chat.ChatMessage) (<-chan StreamChunk, error) {
	// Extract system messages and convert to Anthropic format
	systemPrompt, conversationMessages := a.splitChatMessages(messages)

	temp := DefaultTemperature
	anthropicReq := AnthropicChatRequest{
		Model:       a.modelName,
		MaxTokens:   DefaultMaxTokens,
		Temperature: &temp,
		Messages:    conversationMessages,
		Stream:      true,
	}

	// Add system prompt if we have one
	if systemPrompt != "" {
		anthropicReq.System = systemPrompt
	}

	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicBaseURL+"/messages", bytes.NewBuffer(reqBody))
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
		_ = resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
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
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"user_location": map[string]any{
					"type": "string",
				},
				"scene_change": map[string]any{
					"anyOf": []any{
						map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"to":     map[string]any{"type": "string"},
								"reason": map[string]any{"type": "string"},
							},
							"required": []string{"to", "reason"},
						},
						map[string]any{"type": "null"},
					},
				},
				"item_events": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"item": map[string]any{
								"type": "string",
							},
							"action": map[string]any{
								"type": "string",
								"enum": []string{"acquire", "give", "drop", "move", "use"},
							},
							"from": map[string]any{
								"type":                 "object",
								"additionalProperties": false,
								"properties": map[string]any{
									"type": map[string]any{
										"type": "string",
										"enum": []string{"player", "npc", "location"},
									},
									"name": map[string]any{
										"type": "string",
									},
								},
								"required": []string{"type"},
							},
							"to": map[string]any{
								"type":                 "object",
								"additionalProperties": false,
								"properties": map[string]any{
									"type": map[string]any{
										"type": "string",
										"enum": []string{"player", "npc", "location"},
									},
									"name": map[string]any{
										"type": "string",
									},
								},
								"required": []string{"type"},
							},
							"consumed": map[string]any{
								"type": "boolean",
							},
						},
						"required": []string{"item", "action"},
					},
				},
				"npc_events": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"npc_id": map[string]any{
								"type": "string",
							},
							"set_location": map[string]any{
								"type": "string",
							},
						},
						"required": []string{"npc_id"},
					},
				},
				"set_vars": map[string]any{
					"type": "object",
					"additionalProperties": map[string]any{
						"type": "string",
					},
				},
				"game_ended": map[string]any{
					"type": "boolean",
				},
			},
			"required": []string{"user_location", "scene_change", "item_events", "npc_events", "set_vars", "game_ended"},
		},
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

	deltaUpdate, err := parseDeltaUpdateResponse(content)
	if err != nil {
		return nil, "", err
	}

	return deltaUpdate, modelToUse, nil
}
