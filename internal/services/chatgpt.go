package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

const (
	chatGPTBaseURL = "https://api.openai.com/v1"
)

// ChatGPTService implements LLMService for OpenAI's ChatGPT using the Response API
type ChatGPTService struct {
	apiKey     string
	modelName  string
	httpClient *http.Client
}

// ChatGPTResponseRequest represents the request structure for ChatGPT Response API
type ChatGPTResponseRequest struct {
	Model    string             `json:"model"`
	Messages []chat.ChatMessage `json:"messages"`
	// Response API specific parameters
	Modalities   []string `json:"modalities,omitempty"`   // e.g., ["text"]
	Instructions string   `json:"instructions,omitempty"` // System-level instructions
	// Voice               string   `json:"voice,omitempty"`               // For audio responses
	// OutputAudioFormat   string   `json:"output_audio_format,omitempty"` // e.g., "mp3", "opus", "aac", "flac"
	Temperature         float64 `json:"temperature,omitempty"`
	MaxOutputTokens     int     `json:"max_output_tokens,omitempty"`
	MaxCompletionTokens int     `json:"max_completion_tokens,omitempty"` // Alternative to max_output_tokens
	// Reasoning configuration
	ReasoningEffort string `json:"reasoning_effort,omitempty"` // "low", "medium", "high"
}

// ChatGPTContent represents content in the response (can be text, audio, etc.)
type ChatGPTContent struct {
	Type    string                 `json:"type"` // "text", "audio", etc.
	Text    string                 `json:"text,omitempty"`
	Audio   map[string]interface{} `json:"audio,omitempty"`
	Refusal string                 `json:"refusal,omitempty"`
}

// ChatGPTResponseChoice represents a single choice in the ChatGPT Response API response
type ChatGPTResponseChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string           `json:"role"`
		Content []ChatGPTContent `json:"content"`
		Refusal string           `json:"refusal,omitempty"`
	} `json:"message"`
	Logprobs     interface{} `json:"logprobs,omitempty"`
	FinishReason string      `json:"finish_reason"`
}

// ChatGPTResponseResponse represents the response structure for ChatGPT Response API
type ChatGPTResponseResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []ChatGPTResponseChoice `json:"choices"`
	Usage   struct {
		PromptTokens            int                    `json:"prompt_tokens"`
		CompletionTokens        int                    `json:"completion_tokens"`
		TotalTokens             int                    `json:"total_tokens"`
		PromptTokensDetails     map[string]interface{} `json:"prompt_tokens_details,omitempty"`
		CompletionTokensDetails map[string]interface{} `json:"completion_tokens_details,omitempty"`
		ReasoningTokens         int                    `json:"reasoning_tokens,omitempty"`
	} `json:"usage,omitempty"`
	SystemFingerprint string `json:"system_fingerprint,omitempty"`
	Error             *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
		Param   string `json:"param,omitempty"`
	} `json:"error,omitempty"`
}

// ChatGPTModel represents a model in the ChatGPT models list
type ChatGPTModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ChatGPTModelsResponse represents the response from the ChatGPT models endpoint
type ChatGPTModelsResponse struct {
	Object string         `json:"object"`
	Data   []ChatGPTModel `json:"data"`
	Error  *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewChatGPTService creates a new ChatGPT service using the Response API
func NewChatGPTService(apiKey string, modelName string) *ChatGPTService {
	return &ChatGPTService{
		apiKey:    apiKey,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout: 90 * time.Second, // ChatGPT can be slower than other APIs
		},
	}
}

// InitModel initializes the model (ChatGPT doesn't require explicit model initialization)
func (c *ChatGPTService) InitModel(ctx context.Context, modelName string) error {
	return nil
}

// IsModelReady checks if the model is ready (always true for ChatGPT)
func (c *ChatGPTService) IsModelReady(ctx context.Context, modelName string) (bool, error) {
	return true, nil
}

// ListModels retrieves available models from ChatGPT
func (c *ChatGPTService) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", chatGPTBaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp ChatGPTModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if modelsResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", modelsResp.Error.Message)
	}

	var modelNames []string
	for _, model := range modelsResp.Data {
		modelNames = append(modelNames, model.ID)
	}

	return modelNames, nil
}

// GetChatResponse generates a chat response using the ChatGPT Response API
func (c *ChatGPTService) GetChatResponse(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	// Build the request
	request := ChatGPTResponseRequest{
		Model:           c.modelName,
		Messages:        messages,
		Modalities:      []string{"text"}, // Response API supports multiple modalities
		Temperature:     0.7,
		MaxOutputTokens: 150, // Keep responses concise for roleplay
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", chatGPTBaseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var chatGPTResp ChatGPTResponseResponse
	if err := json.Unmarshal(body, &chatGPTResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if chatGPTResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", chatGPTResp.Error.Message)
	}

	if len(chatGPTResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from API")
	}

	choice := chatGPTResp.Choices[0]

	// Handle refusals
	if choice.Message.Refusal != "" {
		return nil, fmt.Errorf("model refused to respond: %s", choice.Message.Refusal)
	}

	// Extract text content from the response
	var responseText string
	for _, content := range choice.Message.Content {
		if content.Type == "text" {
			responseText = content.Text
			break
		}
		if content.Refusal != "" {
			return nil, fmt.Errorf("content refused: %s", content.Refusal)
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("no text content found in response")
	}

	return &chat.ChatResponse{
		Message: responseText,
	}, nil
}
