package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/state"
)

const (
	// PollInterval is how often to check gamestate for updates
	PollInterval = 1 * time.Second
	// ChatTimeout is max time to wait for chat response to appear
	ChatTimeout = 30 * time.Second
	// DeltaTimeout is max time to wait for DeltaWorker to complete
	DeltaTimeout = 30 * time.Second
	// StoryEventTimeout is max time to wait for a story event to trigger
	StoryEventTimeout = 30 * time.Second
)

// AsyncChatResponse is the response from the async chat endpoint
type AsyncChatResponse struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
}

// PostChatAsync posts a chat message to the async endpoint and returns the request_id
func PostChatAsync(ctx context.Context, client *http.Client, baseURL string, gameStateID uuid.UUID, message string) (string, error) {
	chatReq := map[string]interface{}{
		"gamestate_id": gameStateID.String(),
		"message":      message,
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat", baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send chat request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat endpoint returned %d (expected 202): %s", resp.StatusCode, string(body))
	}

	// Parse response to get request_id
	var chatResp AsyncChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to parse chat response: %w", err)
	}

	return chatResp.RequestID, nil
}

// GetGameState retrieves the current gamestate
func GetGameState(ctx context.Context, client *http.Client, baseURL string, gameStateID uuid.UUID) (*state.GameState, error) {
	url := fmt.Sprintf("%s/v1/gamestate/%s", baseURL, gameStateID.String())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create gamestate request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send gamestate request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gamestate endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var gameState state.GameState
	if err := json.NewDecoder(resp.Body).Decode(&gameState); err != nil {
		return nil, fmt.Errorf("failed to decode gamestate: %w", err)
	}

	return &gameState, nil
}

// PollForChatResponse polls gamestate until chat_history length increases by 2 (user + assistant)
// Returns the updated gamestate and the assistant's response text
func PollForChatResponse(ctx context.Context, client *http.Client, baseURL string, gameStateID uuid.UUID, initialHistoryLen int) (*state.GameState, string, error) {
	timeout := time.After(ChatTimeout)
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-timeout:
			return nil, "", fmt.Errorf("timeout waiting for chat response (waited %v)", ChatTimeout)
		case <-ticker.C:
			gameState, err := GetGameState(ctx, client, baseURL, gameStateID)
			if err != nil {
				// Log error but continue polling
				continue
			}

			// Check if chat_history has increased by at least 2 (user + assistant)
			// Note: It might increase by more if story events triggered
			currentHistoryLen := len(gameState.ChatHistory)
			if currentHistoryLen >= initialHistoryLen+2 {
				// Extract the assistant's response (last message should be assistant)
				var assistantResponse string
				if currentHistoryLen > 0 {
					lastMsg := gameState.ChatHistory[currentHistoryLen-1]
					if lastMsg.Role == "assistant" {
						assistantResponse = lastMsg.Content
					}
				}
				return gameState, assistantResponse, nil
			}
		}
	}
}

// PollForDeltaWorkerCompletion polls gamestate until meta fields update (turn_counter or vars change)
// Returns the final gamestate after DeltaWorker completes
func PollForDeltaWorkerCompletion(ctx context.Context, client *http.Client, baseURL string, gameStateID uuid.UUID, afterChatState *state.GameState) (*state.GameState, error) {
	timeout := time.After(DeltaTimeout)
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	initialTurnCounter := afterChatState.TurnCounter
	initialVarsJSON, _ := json.Marshal(afterChatState.Vars)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for DeltaWorker completion (waited %v)", DeltaTimeout)
		case <-ticker.C:
			gameState, err := GetGameState(ctx, client, baseURL, gameStateID)
			if err != nil {
				// Log error but continue polling
				continue
			}

			// Check if turn_counter changed
			if gameState.TurnCounter != initialTurnCounter {
				return gameState, nil
			}

			// Check if vars changed
			currentVarsJSON, _ := json.Marshal(gameState.Vars)
			if string(currentVarsJSON) != string(initialVarsJSON) {
				return gameState, nil
			}

			// Also check UpdatedAt timestamp as fallback
			if gameState.UpdatedAt.After(afterChatState.UpdatedAt) {
				// Wait one more poll cycle to ensure DeltaWorker fully completed
				time.Sleep(PollInterval)
				finalState, err := GetGameState(ctx, client, baseURL, gameStateID)
				if err != nil {
					return gameState, nil // Return what we have
				}
				return finalState, nil
			}
		}
	}
}
