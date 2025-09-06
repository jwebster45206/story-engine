package services

import (
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
	"github.com/jwebster45206/story-engine/pkg/state"
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
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
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
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
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
			Timeout: 120 * time.Second,
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

// getMetaUpdateTool returns the tool definition for gamestate deltas
func (a *AnthropicService) getMetaUpdateTool() AnthropicTool {
	return AnthropicTool{
		Name:        "apply_changes",
		Description: "Return only the delta for game state updates.",
		InputSchema: map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"user_location": map[string]interface{}{
					"type": "string",
				},
				"scene_name": map[string]interface{}{
					"type": "string",
				},
				"add_to_inventory": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"remove_from_inventory": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"moved_items": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"item": map[string]interface{}{
								"type": "string",
							},
							"from": map[string]interface{}{
								"type": "string",
							},
							"to": map[string]interface{}{
								"type": "string",
							},
							"to_location": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"updated_npcs": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type": "string",
							},
							"description": map[string]interface{}{
								"type": "string",
							},
							"location": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"name"},
					},
				},
				"set_vars": map[string]interface{}{
					"type": "object",
					"additionalProperties": map[string]interface{}{
						"type": "string",
					},
				},
				"game_ended": map[string]interface{}{
					"type": "boolean",
				},
			},
			"required": []string{"user_location", "scene_name", "add_to_inventory", "game_ended"},
		},
	}
}

// MetaUpdate processes a gamestate delta request using Anthropic Claude
func (a *AnthropicService) MetaUpdate(ctx context.Context, messages []chat.ChatMessage) (*state.GameStateDelta, string, error) {
	// Determine which model to use for MetaUpdate
	modelToUse := a.modelName
	if a.backendModelName != "" {
		modelToUse = a.backendModelName
	}

	// Create tools for structured output (first tool will be automatically chosen)
	tools := []AnthropicTool{a.getMetaUpdateTool()}

	content, err := a.chatCompletion(ctx, messages, modelToUse, 0.0, tools)
	if err != nil {
		return nil, "", err
	}

	metaUpdate, err := parseMetaUpdateResponse(content)
	if err != nil {
		return nil, "", err
	}

	return metaUpdate, modelToUse, nil
}
