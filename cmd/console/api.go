package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func testConnection(client *http.Client, baseURL string) bool {
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()
	return resp.StatusCode == http.StatusOK
}

func getGameState(client *http.Client, baseURL string, gameStateID uuid.UUID) (*state.GameState, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v1/gamestate/%s", baseURL, gameStateID))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to get game state: %s", errorResp.Error)
	}

	var gameState state.GameState
	if err := json.Unmarshal(body, &gameState); err != nil {
		return nil, fmt.Errorf("failed to parse game state response: %w", err)
	}
	return &gameState, nil
}

// CreateGameStateRequest matches the API request structure
type CreateGameStateRequest struct {
	Scenario   string `json:"scenario"`
	NarratorID string `json:"narrator_id,omitempty"`
	PCID       string `json:"pc_id,omitempty"`
}

func createGameState(client *http.Client, baseURL string, scenarioFile string, pcID string) (*state.GameState, error) {
	req := CreateGameStateRequest{
		Scenario: scenarioFile,
		PCID:     pcID,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := client.Post(
		baseURL+"/v1/gamestate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to create game state: %s", errorResp.Error)
	}

	var createdGameState state.GameState
	if err := json.Unmarshal(body, &createdGameState); err != nil {
		return nil, fmt.Errorf("failed to parse game state response: %w", err)
	}

	return &createdGameState, nil
}

func listScenarios(client *http.Client, baseURL string) ([]string, map[string]string, error) {
	resp, err := client.Get(baseURL + "/v1/scenarios")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var scenarioMap map[string]string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if err := json.Unmarshal(body, &scenarioMap); err != nil {
		return nil, nil, err
	}

	var names []string
	for name := range scenarioMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, scenarioMap, nil
}

func getScenario(client *http.Client, baseURL string, scenarioFile string) (*scenario.Scenario, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v1/scenarios/%s", baseURL, scenarioFile))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to get scenario: %s", errorResp.Error)
	}

	var scenarioData scenario.Scenario
	if err := json.Unmarshal(body, &scenarioData); err != nil {
		return nil, fmt.Errorf("failed to parse scenario response: %w", err)
	}
	return &scenarioData, nil
}

func listPCs(client *http.Client, baseURL string) ([]string, map[string]string, error) {
	resp, err := client.Get(baseURL + "/v1/pcs")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = resp.Body.Close() // Ignore error in defer
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// API returns an array of PC objects, not a map
	var pcList []map[string]interface{}
	if err := json.Unmarshal(body, &pcList); err != nil {
		return nil, nil, err
	}

	// Build display map: name -> id
	pcMap := make(map[string]string)
	var names []string
	for _, pc := range pcList {
		id, okID := pc["id"].(string)
		name, okName := pc["name"].(string)
		if okID && okName {
			displayName := name
			// Add class/level info if available for better display
			if class, ok := pc["class"].(string); ok && class != "" {
				if level, ok := pc["level"].(float64); ok {
					displayName = fmt.Sprintf("%s (Level %d %s)", name, int(level), class)
				} else {
					displayName = fmt.Sprintf("%s (%s)", name, class)
				}
			}
			names = append(names, displayName)
			pcMap[displayName] = id
		}
	}

	sort.Strings(names)
	return names, pcMap, nil
}

// ChatResponse is the async chat response with request_id
type ChatResponse struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
}

// sendChatAsync sends a chat message and returns the request ID
func sendChatAsync(client *http.Client, baseURL string, gameStateID uuid.UUID, message string) (string, error) {
	reqBody := map[string]interface{}{
		"gamestate_id": gameStateID.String(),
		"message":      message,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := client.Post(
		baseURL+"/v1/chat",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("failed to send chat: %s", errorResp.Error)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return chatResp.RequestID, nil
}

// SSEEvent represents an event from the SSE stream
type SSEEvent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// listenToSSE connects to the SSE endpoint and streams events to a channel
func listenToSSE(ctx context.Context, client *http.Client, baseURL string, gameStateID uuid.UUID, eventChan chan<- SSEEvent) error {
	url := fmt.Sprintf("%s/v1/events/gamestate/%s", baseURL, gameStateID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SSE connection failed with status %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	var currentEvent SSEEvent

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		if line == "" {
			// Empty line signals end of event
			if currentEvent.Type != "" {
				eventChan <- currentEvent
				currentEvent = SSEEvent{}
			}
			continue
		}

		// Parse SSE format
		if strings.HasPrefix(line, "event: ") {
			currentEvent.Type = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataJSON := strings.TrimPrefix(line, "data: ")
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(dataJSON), &data); err == nil {
				currentEvent.Data = data
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading SSE stream: %w", err)
	}

	return nil
}
