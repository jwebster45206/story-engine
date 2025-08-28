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
)

const (
	veniceBaseURL = "https://api.venice.ai/api/v1"
	msgNoResponse = "(no response)"

	DefaultVeniceTemperature = 0.7
	DefaultVeniceMaxTokens   = 2048
)

// VeniceService implements LLMService for Venice AI
type VeniceService struct {
	apiKey     string
	modelName  string
	httpClient *http.Client
}

// VeniceChatRequest represents the request structure for Venice AI chat completions
type VeniceChatRequest struct {
	Model       string             `json:"model"`
	Messages    []chat.ChatMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Stream      bool               `json:"stream"`
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
func NewVeniceService(apiKey string, modelName string) *VeniceService {
	return &VeniceService{
		apiKey:    apiKey,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// InitModel initializes the model (Venice AI doesn't require explicit model initialization)
func (v *VeniceService) InitModel(ctx context.Context, modelName string) error {
	return nil
}

// Chat generates a chat response using Venice AI
func (v *VeniceService) Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	veniceReq := VeniceChatRequest{
		Model:       v.modelName,
		Messages:    messages,
		Temperature: DefaultVeniceTemperature,
		MaxTokens:   DefaultVeniceMaxTokens,
		Stream:      false,
	}

	reqBody, err := json.Marshal(veniceReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", veniceBaseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+v.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var veniceResp VeniceChatResponse
	if err := json.Unmarshal(body, &veniceResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if veniceResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", veniceResp.Error.Message)
	}

	if len(veniceResp.Choices) == 0 {
		return &chat.ChatResponse{
			Message: msgNoResponse,
		}, nil
	}

	return &chat.ChatResponse{
		Message: veniceResp.Choices[0].Message.Content,
	}, nil
}

func (v *VeniceService) MetaUpdate(ctx context.Context, messages []chat.ChatMessage) (*chat.MetaUpdate, error) {
	cr, err := v.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat response: %w", err)
	}

	return parseMetaUpdateResponse(cr.Message)
}
