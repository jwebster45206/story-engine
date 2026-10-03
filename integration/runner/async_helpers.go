package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/state"
)

const (
	// PollInterval is how often to check gamestate for updates
	PollInterval = 1 * time.Second
	// ChatTimeout is max time to wait for chat response to appear
	ChatTimeout = 30 * time.Second
	// DeltaTimeout is max time to wait for the applier to finish
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
	chatReq := map[string]any{
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

// PollForApplier polls gamestate until turn_counter or vars change after a chat turn.
func PollForApplier(ctx context.Context, client *http.Client, baseURL string, gameStateID uuid.UUID, afterChatState *state.GameState) (*state.GameState, error) {
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
			return nil, fmt.Errorf("timeout waiting for applier (waited %v)", DeltaTimeout)
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

			// Also check UpdatedAt timestamp as a fallback
			if gameState.UpdatedAt.After(afterChatState.UpdatedAt) {
				// Wait one more poll cycle so the applier can finish its save
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

// attemptWatch records attempt scopes from the game's SSE stream.
type attemptWatch struct {
	cancel context.CancelFunc
	mu     sync.Mutex
	scopes []string
}

func (w *attemptWatch) stop() {
	if w != nil && w.cancel != nil {
		w.cancel()
	}
}

func (w *attemptWatch) combatCount() int {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, scope := range w.scopes {
		if scope == "combat" {
			n++
		}
	}
	return n
}

func (w *attemptWatch) add(scope string) {
	w.mu.Lock()
	w.scopes = append(w.scopes, scope)
	w.mu.Unlock()
}

// watchAttempts subscribes to GET /v1/events/gamestate/{id} and returns once
// the stream is connected. Call stop when the step is finished.
func (r *Runner) watchAttempts(parent context.Context, gameStateID uuid.UUID) (*attemptWatch, error) {
	ctx, cancel := context.WithCancel(parent)
	w := &attemptWatch{cancel: cancel}
	ready := make(chan struct{})
	errc := make(chan error, 1)
	go func() {
		err := readAttemptStream(ctx, r.sseClient(), r.BaseURL, gameStateID, w, ready)
		if err != nil && ctx.Err() == nil {
			errc <- err
		}
	}()

	select {
	case <-parent.Done():
		cancel()
		return nil, parent.Err()
	case err := <-errc:
		cancel()
		return nil, err
	case <-ready:
		return w, nil
	case <-time.After(10 * time.Second):
		cancel()
		return nil, fmt.Errorf("timeout waiting for event stream")
	}
}

func (r *Runner) sseClient() *http.Client {
	return &http.Client{Transport: r.Client.Transport}
}

func readAttemptStream(ctx context.Context, client *http.Client, baseURL string, gameStateID uuid.UUID, watch *attemptWatch, ready chan struct{}) (err error) {
	url := fmt.Sprintf("%s/v1/events/gamestate/%s", baseURL, gameStateID.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create event stream request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to event stream: %w", err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if err == nil && closeErr != nil && ctx.Err() == nil {
			err = fmt.Errorf("close event stream: %w", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("event stream returned %d", resp.StatusCode)
		}
		return fmt.Errorf("event stream returned %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventType string
	var dataLine string
	connected := false
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Text()
		if line == "" {
			if eventType == "connected" && !connected {
				connected = true
				close(ready)
			}
			if eventType == "attempt" && dataLine != "" {
				var data map[string]any
				if err := json.Unmarshal([]byte(dataLine), &data); err != nil {
					return fmt.Errorf("decode attempt event: %w", err)
				}
				scope, _ := data["scope"].(string)
				watch.add(scope)
			}
			eventType = ""
			dataLine = ""
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if value, ok := strings.CutPrefix(line, "event: "); ok {
			eventType = value
			continue
		}
		if value, ok := strings.CutPrefix(line, "data: "); ok {
			dataLine = value
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read event stream: %w", err)
	}
	if !connected {
		return fmt.Errorf("event stream closed before connect")
	}
	return nil
}
