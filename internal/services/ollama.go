package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

// OllamaService implements the LLMService interface for Ollama API
type OllamaService struct {
	baseURL    string
	modelName  string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewOllamaService creates a new Ollama service instance
func NewOllamaService(baseURL string, modelName string, logger *slog.Logger) *OllamaService {
	return &OllamaService{
		baseURL:   baseURL,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// InitModel initializes the LLM model by checking if it's available
func (s *OllamaService) InitModel(ctx context.Context, modelName string) error {
	s.logger.Info("Initializing LLM model", "model", modelName)

	if err := s.waitForOllamaReady(ctx); err != nil {
		return fmt.Errorf("ollama service is not ready: %w", err)
	}

	ready, err := s.IsModelReady(ctx, modelName)
	if err != nil {
		return fmt.Errorf("failed to check model readiness: %w", err)
	}

	if !ready {
		// Pull the model if it's not available
		s.logger.Info("Model not found, pulling it", "model", modelName)
		if err := s.pullModel(ctx, modelName); err != nil {
			return fmt.Errorf("failed to pull model: %w", err)
		}
		s.logger.Info("Model pulled successfully", "model", modelName)
	} else {
		s.logger.Info("Model already available", "model", modelName)
	}

	return nil
}

// GenerateResponse generates a chat response using the Ollama API
func (s *OllamaService) GenerateResponse(ctx context.Context, messages []chat.ChatMessage) (*chat.ChatResponse, error) {
	reqBody := map[string]interface{}{
		"model":    s.modelName,
		"messages": messages,
		"stream":   false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/api/chat", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chat.ChatResponse{
		Message: ollamaResp.Message.Content,
	}, nil
}

// IsModelReady checks if the specified model is available
func (s *OllamaService) IsModelReady(ctx context.Context, modelName string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/api/tags", nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, model := range tagsResp.Models {
		if model.Name == modelName {
			return true, nil
		}
	}

	return false, nil
}

// pullModel pulls a model from Ollama
func (s *OllamaService) pullModel(ctx context.Context, modelName string) error {
	reqBody := map[string]string{
		"name": modelName,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/api/pull", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Use a longer timeout for pulling models as it can take a while
	client := &http.Client{
		Timeout: 10 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	return nil
}

// waitForOllamaReady waits for Ollama service to be ready with retries
func (s *OllamaService) waitForOllamaReady(ctx context.Context) error {
	maxRetries := 5
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/api/tags", nil)
		if err != nil {
			s.logger.Debug("Failed to create request for readiness check", "error", err, "attempt", i+1)
			time.Sleep(retryDelay)
			continue
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			s.logger.Debug("Ollama not ready yet", "error", err, "attempt", i+1)
			time.Sleep(retryDelay)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			s.logger.Info("Ollama service is ready")
			return nil
		}

		s.logger.Debug("Ollama returned non-200 status", "status", resp.StatusCode, "attempt", i+1)
		time.Sleep(retryDelay)
	}

	return fmt.Errorf("ollama service did not become ready after %d attempts", maxRetries)
}
