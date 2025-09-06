package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/state"
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
	Name   string                 `json:"name"`
	Strict bool                   `json:"strict"`
	Schema map[string]interface{} `json:"schema"`
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
			Schema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"user_location": map[string]interface{}{
						"type": "string",
					},
					"scene_change": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]interface{}{
							"to": map[string]interface{}{
								"type": "string",
							},
							"reason": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"to", "reason"},
					},
					"item_events": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]interface{}{
								"item": map[string]interface{}{
									"type": "string",
								},
								"action": map[string]interface{}{
									"type": "string",
									"enum": []string{"acquire", "give", "drop", "move", "use"},
								},
								"from": map[string]interface{}{
									"type":                 "object",
									"additionalProperties": false,
									"properties": map[string]interface{}{
										"type": map[string]interface{}{
											"type": "string",
											"enum": []string{"player", "npc", "location"},
										},
										"name": map[string]interface{}{
											"type": "string",
										},
									},
									"required": []string{"type"},
								},
								"to": map[string]interface{}{
									"type":                 "object",
									"additionalProperties": false,
									"properties": map[string]interface{}{
										"type": map[string]interface{}{
											"type": "string",
											"enum": []string{"player", "npc", "location"},
										},
										"name": map[string]interface{}{
											"type": "string",
										},
									},
									"required": []string{"type"},
								},
								"consumed": map[string]interface{}{
									"type": "boolean",
								},
							},
							"required": []string{"item", "action"},
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

func (v *VeniceService) DeltaUpdate(ctx context.Context, messages []chat.ChatMessage) (*state.GameStateDelta, string, error) {
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
