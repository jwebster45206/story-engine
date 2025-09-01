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
)

const (
	anthropicBaseURL = "https://api.anthropic.com/v1"
	anthropicVersion = "2023-06-01"

	DefaultAnthropicTemperature = 0.7
	DefaultAnthropicMaxTokens   = 2048
)

// AnthropicService implements LLMService for Anthropic Claude
type AnthropicService struct {
	apiKey           string
	modelName        string
	backendModelName string
	httpClient       *http.Client
	logger           *slog.Logger
}

type AnthropicChatRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   *float64           `json:"temperature,omitempty"`
	Messages      []chat.ChatMessage `json:"messages"`
	System        string             `json:"system,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	TopK          *int               `json:"top_k,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
}

type AnthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
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
func (a *AnthropicService) chatCompletion(ctx context.Context, messages []chat.ChatMessage, modelName string) (string, error) {
	// Extract system messages and convert to Anthropic format
	systemPrompt, conversationMessages := a.splitChatMessages(messages)

	temperature := DefaultAnthropicTemperature
	anthropicReq := AnthropicChatRequest{
		Model:       modelName,
		MaxTokens:   DefaultAnthropicMaxTokens,
		Temperature: &temperature,
		Messages:    conversationMessages,
		Stream:      false,
	}

	// Add system prompt if we have one
	if systemPrompt != "" {
		anthropicReq.System = systemPrompt
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

	// Extract text content from the response
	var responseText string
	for _, content := range anthropicResp.Content {
		if content.Type == "text" {
			responseText += content.Text
		}
	}

	if responseText == "" {
		responseText = "(no response)"
	}

	return responseText, nil
}

func (a *AnthropicService) Chat(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	content, err := a.chatCompletion(ctx, messages, a.modelName)
	if err != nil {
		return nil, err
	}

	return &chat.ChatResponse{
		Message: content,
	}, nil
}

// MetaUpdate processes a meta update request using Anthropic Claude
func (a *AnthropicService) MetaUpdate(ctx context.Context, messages []chat.ChatMessage) (*chat.MetaUpdate, string, error) {
	// Determine which model to use for MetaUpdate
	modelToUse := a.modelName
	if a.backendModelName != "" {
		modelToUse = a.backendModelName
	}

	content, err := a.chatCompletion(ctx, messages, modelToUse)
	if err != nil {
		return nil, "", err
	}

	metaUpdate, err := parseMetaUpdateResponse(content)
	if err != nil {
		return nil, "", err
	}

	return metaUpdate, modelToUse, nil
}
