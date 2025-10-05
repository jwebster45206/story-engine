package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/jwebster45206/story-engine/pkg/state"
)

// Runner executes integration tests against a running story-engine API
type Runner struct {
	BaseURL string
	Client  *http.Client
	Timeout time.Duration
	Logger  func(format string, args ...interface{})
}

// NewRunner creates a new test runner
func NewRunner(baseURL string) *Runner {
	return &Runner{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Client:  &http.Client{Timeout: 60 * time.Second},
		Timeout: 30 * time.Second,
	}
}

// LoadTestSuite loads a test suite from a YAML file
func LoadTestSuite(filename string) (TestSuite, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return TestSuite{}, fmt.Errorf("failed to read test file %s: %w", filename, err)
	}

	var suite TestSuite
	if err := yaml.Unmarshal(content, &suite); err != nil {
		return TestSuite{}, fmt.Errorf("failed to parse YAML in %s: %w", filename, err)
	}

	return suite, nil
}

// RunSuite executes a complete test suite
func (r *Runner) RunSuite(ctx context.Context, suite TestSuite) (TestRunResult, error) {
	start := time.Now()
	result := TestRunResult{
		Job: TestJob{
			Name:  suite.Name,
			Suite: suite,
		},
		Results: make([]TestResult, 0, len(suite.Steps)),
	}

	// Generate unique gamestate ID for this test run
	gameStateID := uuid.New()
	result.GameState = gameStateID

	// Seed the gamestate (creates a new one)
	actualGameStateID, err := r.seedGameState(ctx, suite.SeedGameState)
	if err != nil {
		result.Error = fmt.Errorf("failed to seed gamestate: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}

	// Update result to use the actual gamestate ID
	gameStateID = actualGameStateID
	result.GameState = gameStateID

	// Track state across steps for relative expectations
	prevTurnCounter := suite.SeedGameState.TurnCounter
	prevInventory := make([]string, len(suite.SeedGameState.Inventory))
	copy(prevInventory, suite.SeedGameState.Inventory)

	// Execute each test step
	for i, step := range suite.Steps {
		r.Logger("    [%d/%d] Running step: %s", i+1, len(suite.Steps), step.Name)
		stepResult := r.runStep(ctx, gameStateID, step, prevTurnCounter, prevInventory)
		result.Results = append(result.Results, stepResult)

		if stepResult.Error != nil {
			r.Logger("    [%d/%d] ✗ %s: %v", i+1, len(suite.Steps), step.Name, stepResult.Error)
			result.Error = fmt.Errorf("step %d (%s) failed: %w", i, step.Name, stepResult.Error)
			break
		}

		r.Logger("    [%d/%d] ✓ %s (%v)", i+1, len(suite.Steps), step.Name, stepResult.Duration)

		// Update tracking state for next step
		if stepResult.Success {
			gameState, err := r.getGameState(ctx, gameStateID)
			if err != nil {
				result.Error = fmt.Errorf("failed to get updated gamestate after step %d: %w", i, err)
				break
			}
			prevTurnCounter = gameState.TurnCounter
			prevInventory = make([]string, len(gameState.Inventory))
			copy(prevInventory, gameState.Inventory)
		}
	}

	result.Duration = time.Since(start)
	return result, result.Error
}

// seedGameState creates a new gamestate and then patches it with seed data
func (r *Runner) seedGameState(ctx context.Context, seed state.GameState) (uuid.UUID, error) {
	// Step 1: Create a basic gamestate via POST /v1/gamestate
	// ModelName is set by the handler from its configuration, Scenario is set from request
	createReq := state.GameState{
		Scenario: seed.Scenario,
	}

	createBody, err := json.Marshal(createReq)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to marshal create request: %w", err)
	}

	createURL := r.BaseURL + "/v1/gamestate"
	req, err := http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewBuffer(createBody))
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.Client.Do(req)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to create gamestate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return uuid.UUID{}, fmt.Errorf("create gamestate returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response to get the created gamestate
	var createdGS state.GameState
	if err := json.NewDecoder(resp.Body).Decode(&createdGS); err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to decode created gamestate: %w", err)
	}

	// Step 2: PATCH the gamestate with our seed data
	// Create patch data (excluding immutable fields like ModelName and Scenario)
	patchData := state.GameState{
		// ModelName and Scenario are NOT patchable - they're set at creation time
		SceneName:        seed.SceneName,
		Location:         seed.Location,
		TurnCounter:      seed.TurnCounter,
		SceneTurnCounter: seed.SceneTurnCounter,
		Inventory:        seed.Inventory,
		Vars:             seed.Vars,
		ChatHistory:      seed.ChatHistory, // No conversion needed - same type
		IsEnded:          seed.IsEnded,
	}

	patchBody, err := json.Marshal(patchData)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to marshal patch request: %w", err)
	}

	// PATCH the created gamestate with our seed data
	patchURL := fmt.Sprintf("%s/v1/gamestate/%s", r.BaseURL, createdGS.ID.String())
	patchReq, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, bytes.NewBuffer(patchBody))
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to create PATCH request: %w", err)
	}
	patchReq.Header.Set("Content-Type", "application/json")

	patchResp, err := r.Client.Do(patchReq)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to patch gamestate: %w", err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(patchResp.Body)
		return uuid.UUID{}, fmt.Errorf("patch gamestate returned %d: %s", patchResp.StatusCode, string(body))
	}

	// Return the ID of the successfully seeded gamestate
	return createdGS.ID, nil
}

// runStep executes a single test step and checks expectations
func (r *Runner) runStep(ctx context.Context, gameStateID uuid.UUID, step TestStep, prevTurnCounter int, prevInventory []string) TestResult {
	start := time.Now()
	result := TestResult{
		StepName: step.Name,
	}

	// Get gamestate before chat for expectations comparison
	preGameState, err := r.getGameState(ctx, gameStateID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get gamestate before chat: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Send chat message
	chatResp, err := r.sendChatMessage(ctx, gameStateID, step.UserPrompt)
	if err != nil {
		result.Error = fmt.Errorf("failed to send chat message: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	result.ResponseText = chatResp

	// Capture timestamp AFTER receiving chat response (like ui.go does)
	chatCompletedAt := time.Now()

	// Wait for gamestate to be updated (poll until UpdatedAt is after chat completion)
	postGameState, err := r.waitForGameStateUpdate(ctx, gameStateID, chatCompletedAt)
	if err != nil {
		result.Error = fmt.Errorf("failed to wait for gamestate update: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Check expectations
	if err := r.checkExpectations(step.Expectations, preGameState, postGameState, prevTurnCounter, prevInventory, chatResp); err != nil {
		result.Error = fmt.Errorf("expectation failed: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Success = true
	result.Duration = time.Since(start)
	return result
}

// sendChatMessage sends a chat message and returns the response content
func (r *Runner) sendChatMessage(ctx context.Context, gameStateID uuid.UUID, message string) (string, error) {
	chatReq := map[string]interface{}{
		"gamestate_id": gameStateID.String(),
		"message":      message,
		"stream":       false, // Use non-streaming for simpler testing
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat", r.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to get the message content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read chat response: %w", err)
	}

	var chatResp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse chat response: %w", err)
	}

	return chatResp.Message, nil
}

// getGameState retrieves the current gamestate
func (r *Runner) getGameState(ctx context.Context, gameStateID uuid.UUID) (*state.GameState, error) {
	url := fmt.Sprintf("%s/v1/gamestate/%s", r.BaseURL, gameStateID.String())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create gamestate request: %w", err)
	}

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send gamestate request: %w", err)
	}
	defer resp.Body.Close()

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

// waitForGameStateUpdate polls until the gamestate UpdatedAt field changes
func (r *Runner) waitForGameStateUpdate(ctx context.Context, gameStateID uuid.UUID, preUpdateTime time.Time) (*state.GameState, error) {
	timeout := time.After(r.Timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for gamestate update")
		case <-ticker.C:
			gameState, err := r.getGameState(ctx, gameStateID)
			if err != nil {
				continue // Retry on error
			}
			if gameState.UpdatedAt.After(preUpdateTime) {
				return gameState, nil
			}
		}
	}
}

// checkExpectations validates the test expectations against the actual gamestate changes
func (r *Runner) checkExpectations(exp Expectations, preState, postState *state.GameState, prevTurnCounter int, prevInventory []string, responseText string) error {
	// Location check
	if exp.Location != nil {
		if postState.Location != *exp.Location {
			return fmt.Errorf("expected location %s, got %s", *exp.Location, postState.Location)
		}
	}

	// Scene check
	if exp.SceneName != nil {
		if postState.SceneName != *exp.SceneName {
			return fmt.Errorf("expected scene %s, got %s", *exp.SceneName, postState.SceneName)
		}
	}

	// Inventory additions check
	if len(exp.InventoryAdded) > 0 {
		for _, expectedItem := range exp.InventoryAdded {
			found := false
			for _, item := range postState.Inventory {
				if item == expectedItem {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("expected item %s to be added to inventory, but it wasn't found", expectedItem)
			}
			// Also verify it wasn't in the previous inventory
			prevHad := false
			for _, item := range prevInventory {
				if item == expectedItem {
					prevHad = true
					break
				}
			}
			if prevHad {
				return fmt.Errorf("expected item %s to be added, but it was already in inventory", expectedItem)
			}
		}
	}

	// Inventory removal check
	if len(exp.InventoryRemoved) > 0 {
		for _, expectedRemovedItem := range exp.InventoryRemoved {
			// Verify it was in previous inventory
			prevHad := false
			for _, item := range prevInventory {
				if item == expectedRemovedItem {
					prevHad = true
					break
				}
			}
			if !prevHad {
				return fmt.Errorf("expected item %s to be removed, but it wasn't in previous inventory", expectedRemovedItem)
			}
			// Verify it's not in current inventory
			found := false
			for _, item := range postState.Inventory {
				if item == expectedRemovedItem {
					found = true
					break
				}
			}
			if found {
				return fmt.Errorf("expected item %s to be removed from inventory, but it's still there", expectedRemovedItem)
			}
		}
	}

	// Variables check
	if len(exp.Vars) > 0 {
		for key, expectedValue := range exp.Vars {
			actualValue, exists := postState.Vars[key]
			if !exists {
				return fmt.Errorf("expected variable %s to be set, but it doesn't exist", key)
			}
			if actualValue != expectedValue {
				return fmt.Errorf("expected variable %s to be %s, got %s", key, expectedValue, actualValue)
			}
		}
	}

	// NPC locations check
	if len(exp.NPCLocations) > 0 {
		for npcName, expectedLocation := range exp.NPCLocations {
			npc, exists := postState.NPCs[npcName]
			if !exists {
				return fmt.Errorf("expected NPC %s to exist, but it doesn't", npcName)
			}
			if npc.Location != expectedLocation {
				return fmt.Errorf("expected NPC %s to be at %s, got %s", npcName, expectedLocation, npc.Location)
			}
		}
	}

	// Response content checks
	if len(exp.ResponseContains) > 0 {
		lowerResponse := strings.ToLower(responseText)
		for _, expectedText := range exp.ResponseContains {
			if !strings.Contains(lowerResponse, strings.ToLower(expectedText)) {
				return fmt.Errorf("expected response to contain '%s', but it didn't", expectedText)
			}
		}
	}

	if len(exp.ResponseNotContains) > 0 {
		lowerResponse := strings.ToLower(responseText)
		for _, unexpectedText := range exp.ResponseNotContains {
			if strings.Contains(lowerResponse, strings.ToLower(unexpectedText)) {
				return fmt.Errorf("expected response to NOT contain '%s', but it did", unexpectedText)
			}
		}
	}

	// Regex check
	if exp.ResponseRegex != "" {
		matched, err := regexp.MatchString(exp.ResponseRegex, responseText)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		if !matched {
			return fmt.Errorf("response didn't match regex pattern: %s", exp.ResponseRegex)
		}
	}

	// Response length checks
	if exp.ResponseMinLength != nil {
		if len(responseText) < *exp.ResponseMinLength {
			return fmt.Errorf("expected response length >= %d, got %d", *exp.ResponseMinLength, len(responseText))
		}
	}
	if exp.ResponseMaxLength != nil {
		if len(responseText) > *exp.ResponseMaxLength {
			return fmt.Errorf("expected response length <= %d, got %d", *exp.ResponseMaxLength, len(responseText))
		}
	}

	// Game ended check
	if exp.GameEnded != nil {
		if postState.IsEnded != *exp.GameEnded {
			return fmt.Errorf("expected game ended to be %t, got %t", *exp.GameEnded, postState.IsEnded)
		}
	}

	// Turn increment check
	if exp.TurnIncrement != nil {
		expectedTurnCount := prevTurnCounter + *exp.TurnIncrement
		if postState.TurnCounter != expectedTurnCount {
			return fmt.Errorf("expected turn counter to be %d (prev %d + %d), got %d",
				expectedTurnCount, prevTurnCounter, *exp.TurnIncrement, postState.TurnCounter)
		}
	}

	return nil
}
