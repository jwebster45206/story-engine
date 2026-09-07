package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

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

func createGameState(client *http.Client, baseURL string, scenarioFile string, pcID string, provider string, rules string, temperature float64) (*state.GameState, error) {
	req := state.GameState{
		Scenario:    scenarioFile,
		Provider:    provider,
		Rules:       state.RulesMode(rules),
		Temperature: temperature,
	}
	if pcID != "" {
		req.PC = &actor.PC{Spec: &actor.PCSpec{ID: pcID}}
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
	var pcList []map[string]any
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

type providerOption struct {
	Name        string
	DisplayName string
	Vendor      string
	Model       string
}

type providersListResponse struct {
	Default   string `json:"default"`
	Providers []struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Vendor      string `json:"vendor"`
		Model       string `json:"model"`
	} `json:"providers"`
}

func getProviders(client *http.Client, baseURL string) (string, []providerOption, error) {
	resp, err := client.Get(baseURL + "/v1/providers")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var parsed providersListResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", nil, err
	}

	opts := make([]providerOption, 0, len(parsed.Providers))
	for _, p := range parsed.Providers {
		display := p.DisplayName
		if display == "" {
			display = p.Name
		}
		opts = append(opts, providerOption{
			Name:        p.Name,
			DisplayName: display,
			Vendor:      p.Vendor,
			Model:       p.Model,
		})
	}
	return parsed.Default, opts, nil
}

// ChatResponse is the async chat response with request_id
type ChatResponse struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
}

// sendChatAsync sends a chat message and returns the request ID
func sendChatAsync(client *http.Client, baseURL string, gameStateID uuid.UUID, message string) (string, error) {
	reqBody := map[string]any{
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
