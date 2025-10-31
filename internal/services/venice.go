package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

const (
	veniceBaseURL = "https://api.venice.ai/api/v1"
	msgNoResponse = "(no response)"
)

// VeniceService implements LLMService for Venice AI
type VeniceService struct {
	apiKey           string
	modelName        string
	backendModelName string
	httpClient       *http.Client
}

type VeniceResponseFormat struct {
	Type       string           `json:"type"`
	JSONSchema VeniceJSONSchema `json:"json_schema"`
}

type VeniceJSONSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type VeniceParameters struct {
	IncludeVeniceSystemPrompt bool   `json:"include_venice_system_prompt"`
	EnableWebSearch           string `json:"enable_web_search"`
}

// VeniceChatRequest represents the request structure for Venice AI chat completions
type VeniceChatRequest struct {
	Model            string                `json:"model"`
	Messages         []chat.ChatMessage    `json:"messages"`
	Temperature      float64               `json:"temperature,omitempty"`
	MaxTokens        int                   `json:"max_tokens,omitempty"`
	Stream           bool                  `json:"stream"`
	ResponseFormat   *VeniceResponseFormat `json:"response_format,omitempty"`
	VeniceParameters VeniceParameters      `json:"venice_parameters"`
}

// VeniceChatChoice represents a single choice in the Venice AI response
type VeniceChatChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// VeniceChatResponse represents the response structure for Venice AI chat completions
type VeniceChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []VeniceChatChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// VeniceStreamChoice represents a streaming choice in Venice AI response
type VeniceStreamChoice struct {
	Index int `json:"index"`
	Delta struct {
		Role    string `json:"role,omitempty"`
		Content string `json:"content,omitempty"`
	} `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// VeniceStreamResponse represents the streaming response structure for Venice AI
type VeniceStreamResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []VeniceStreamChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewVeniceService creates a new Venice AI service
func NewVeniceService(apiKey string, modelName string, backendModelName string) *VeniceService {
	return &VeniceService{
		apiKey:           apiKey,
		modelName:        modelName,
		backendModelName: backendModelName,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// InitModel initializes the model (Venice AI doesn't require explicit model initialization)
func (v *VeniceService) InitModel(ctx context.Context, modelName string) error {
	return nil
}

// chatCompletion makes a chat completion request to Venice AI with the specified model
func (v *VeniceService) chatCompletion(ctx context.Context, messages []chat.ChatMessage, modelName string, temperature float64, responseFormat *VeniceResponseFormat) (string, error) {
	maxTokens := DefaultMaxTokens
	if temperature == 0.0 {
		maxTokens = BackendMaxTokens
	}
	veniceReq := VeniceChatRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      false,
		VeniceParameters: VeniceParameters{
			IncludeVeniceSystemPrompt: false,
			EnableWebSearch:           "off",
		},
	}

	// Add response format if provided
	if responseFormat != nil {
		veniceReq.ResponseFormat = responseFormat
	}

	reqBody, err := json.Marshal(veniceReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", veniceBaseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+v.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
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

	var veniceResp VeniceChatResponse
	if err := json.Unmarshal(body, &veniceResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if veniceResp.Error != nil {
		return "", fmt.Errorf("API error: %s", veniceResp.Error.Message)
	}

	if len(veniceResp.Choices) == 0 {
		return msgNoResponse, nil
	}

	return veniceResp.Choices[0].Message.Content, nil
}

// getDeltaUpdateResponseFormat returns the response format
// for structured gamestate updates
func (v *VeniceService) getDeltaUpdateResponseFormat() *VeniceResponseFormat {
	return &VeniceResponseFormat{
		Type: "json_schema",
		JSONSchema: VeniceJSONSchema{
			Name:   "apply_changes",
			Strict: true,
			Schema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"user_location": map[string]any{
						"type": "string",
					},
					// REQUIRED + NULLABLE scene_change
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
		},
	}
}

// Chat generates a chat response using Venice AI
func (v *VeniceService) Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	content, err := v.chatCompletion(ctx, messages, v.modelName, DefaultTemperature, nil)
	if err != nil {
		return nil, err
	}

	return &chat.ChatResponse{
		Message: content,
	}, nil
}

// ChatStream generates a streaming chat response using Venice AI
func (v *VeniceService) ChatStream(ctx context.Context, messages []chat.ChatMessage) (<-chan StreamChunk, error) {
	reqBody := VeniceChatRequest{
		Model:       v.modelName,
		Messages:    messages,
		Temperature: DefaultTemperature,
		MaxTokens:   DefaultMaxTokens,
		Stream:      true,
		VeniceParameters: VeniceParameters{
			IncludeVeniceSystemPrompt: false,
			EnableWebSearch:           "off",
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", veniceBaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.httpClient.Do(req)
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

			// Venice streaming responses are in SSE format: "data: {json}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			// Remove "data: " prefix
			jsonData := strings.TrimPrefix(line, "data: ")

			// Check for end of stream
			if jsonData == "[DONE]" {
				chunkChan <- StreamChunk{Done: true}
				return
			}

			var streamResp VeniceStreamResponse
			if err := json.Unmarshal([]byte(jsonData), &streamResp); err != nil {
				chunkChan <- StreamChunk{Error: fmt.Errorf("failed to decode streaming response: %w", err)}
				return
			}

			// Check for API errors
			if streamResp.Error != nil {
				chunkChan <- StreamChunk{Error: fmt.Errorf("venice API error: %s", streamResp.Error.Message)}
				return
			}

			// Extract content from the first choice
			if len(streamResp.Choices) > 0 {
				choice := streamResp.Choices[0]
				chunkChan <- StreamChunk{
					Content: choice.Delta.Content,
					Done:    choice.FinishReason != nil,
				}

				// Check if streaming is complete
				if choice.FinishReason != nil {
					chunkChan <- StreamChunk{Done: true}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			chunkChan <- StreamChunk{Error: fmt.Errorf("error reading stream: %w", err)}
		}
	}()

	return chunkChan, nil
}

func (v *VeniceService) DeltaUpdate(ctx context.Context, messages []chat.ChatMessage) (*conditionals.GameStateDelta, string, error) {
	modelToUse := v.modelName
	if v.backendModelName != "" {
		modelToUse = v.backendModelName
	}

	// Use structured JSON response format with temperature 0 for deterministic output
	responseFormat := v.getDeltaUpdateResponseFormat()
	content, err := v.chatCompletion(ctx, messages, modelToUse, 0.0, responseFormat)
	if err != nil {
		return nil, "", err
	}

	deltaUpdate, err := parseDeltaUpdateResponse(content)
	if err != nil {
		return nil, "", err
	}

	return deltaUpdate, modelToUse, nil
}
