package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jwebster45206/story-engine/pkg/state"
)

type ErrorHandlingMode string

const ErrorHandlingExit ErrorHandlingMode = "exit"
const ErrorHandlingContinue ErrorHandlingMode = "continue"

// Runner executes integration tests against a running story-engine API
type Runner struct {
	BaseURL           string
	Client            *http.Client
	Timeout           time.Duration
	Logger            func(format string, args ...interface{})
	ErrorHandlingMode ErrorHandlingMode
	ScenarioOverride  string // If set, overrides the scenario for all test cases
}

// NewRunner creates a new test runner
func NewRunner(baseURL string) *Runner {
	return &Runner{
		BaseURL:           strings.TrimSuffix(baseURL, "/"),
		Client:            &http.Client{Timeout: 60 * time.Second},
		Timeout:           30 * time.Second,
		ErrorHandlingMode: ErrorHandlingContinue,
	}
}

// LoadTestSuite loads a test suite from a JSON file
func LoadTestSuite(filename string) (TestSuite, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return TestSuite{}, fmt.Errorf("failed to read test file %s: %w", filename, err)
	}

	var suite TestSuite
	if err := json.Unmarshal(content, &suite); err != nil {
		return TestSuite{}, fmt.Errorf("failed to parse JSON in %s: %w", filename, err)
	}

	return suite, nil
}

// LoadTestSuiteWithExpansion loads a test suite and expands it if it's a sequence
// Returns a list of actual test suites (expanded from the sequence if needed)
func LoadTestSuiteWithExpansion(filename string, casesDir string) ([]TestJob, error) {
	suite, err := LoadTestSuite(filename)
	if err != nil {
		return nil, err
	}

	// If this is not a sequence, return it as-is
	if !suite.IsSequence() {
		return []TestJob{{
			Name:     suite.Name,
			Suite:    suite,
			CaseFile: filename,
		}}, nil
	}

	// This is a sequence - load all referenced cases
	var jobs []TestJob
	for _, caseFile := range suite.Cases {
		// Resolve path relative to casesDir
		casePath := filepath.Join(casesDir, caseFile)

		// Recursively load (in case a sequence references another sequence)
		subJobs, err := LoadTestSuiteWithExpansion(casePath, casesDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load case '%s' referenced by sequence '%s': %w", caseFile, suite.Name, err)
		}

		jobs = append(jobs, subJobs...)
	}

	return jobs, nil
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
	seedData := suite.SeedGameState
	// Use scenario override if provided, otherwise use suite-level scenario
	if r.ScenarioOverride != "" {
		seedData.Scenario = r.ScenarioOverride
	} else {
		seedData.Scenario = suite.Scenario
	}
	actualGameStateID, err := r.seedGameState(ctx, seedData)
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
		stepResult := r.runStep(ctx, gameStateID, step, prevTurnCounter, prevInventory, &suite.SeedGameState)
		result.Results = append(result.Results, stepResult)

		if stepResult.Error != nil {
			r.Logger("    [%d/%d] ✗ %s: %v", i+1, len(suite.Steps), step.Name, stepResult.Error)
			if result.Error == nil {
				result.Error = fmt.Errorf("step %d (%s) failed: %w", i, step.Name, stepResult.Error)
			}
			// Break only if error handling mode is "exit"
			if r.ErrorHandlingMode == ErrorHandlingExit {
				break
			}
			// Continue to next step if mode is "continue"
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return uuid.UUID{}, fmt.Errorf("create gamestate returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response to get the created gamestate
	var createdGS state.GameState
	if err := json.NewDecoder(resp.Body).Decode(&createdGS); err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to decode created gamestate: %w", err)
	}

	// Step 2: Use resetGameState to apply all seed data consistently
	if err := r.resetGameState(ctx, createdGS.ID, &seed); err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to seed gamestate: %w", err)
	}

	// Return the ID of the successfully seeded gamestate
	return createdGS.ID, nil
}

// resetGameState resets the gamestate to the original seed data
func (r *Runner) resetGameState(ctx context.Context, gameStateID uuid.UUID, seedState *state.GameState) error {
	// Use the same PATCH logic as seedGameState, but exclude immutable fields
	patchData := state.GameState{
		SceneName:          seedState.SceneName,
		Location:           seedState.Location,
		TurnCounter:        seedState.TurnCounter,
		SceneTurnCounter:   seedState.SceneTurnCounter,
		Inventory:          seedState.Inventory,
		Vars:               seedState.Vars,
		ChatHistory:        seedState.ChatHistory,
		IsEnded:            seedState.IsEnded,
		NPCs:               seedState.NPCs,
		WorldLocations:     seedState.WorldLocations,
		ContingencyPrompts: seedState.ContingencyPrompts,
	}

	patchBody, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal reset patch data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", r.BaseURL+"/v1/gamestate/"+gameStateID.String(), bytes.NewReader(patchBody))
	if err != nil {
		return fmt.Errorf("failed to create PATCH request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute PATCH request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PATCH request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// runStep executes a single test step and checks expectations
// If step.UserPrompt is ResetGameStatePrompt, resets the gamestate to seedState
// Will retry once on timeout errors without backoff
func (r *Runner) runStep(ctx context.Context, gameStateID uuid.UUID, step TestStep, prevTurnCounter int, prevInventory []string, seedState *state.GameState) TestResult {
	// Try once, then retry on timeout
	for attempt := 1; attempt <= 2; attempt++ {
		result := r.executeStep(ctx, gameStateID, step, prevTurnCounter, prevInventory, seedState)

		// If successful or not a timeout, return immediately
		if result.Success || result.Error == nil {
			return result
		}

		// Check if it's a timeout error
		isTimeout := strings.Contains(result.Error.Error(), "timeout waiting for gamestate update")

		// If it's a timeout and this is the first attempt, retry
		if isTimeout && attempt == 1 {
			r.Logger("    Timeout detected, retrying step: %s", step.Name)
			continue
		}

		// Otherwise, return the result
		return result
	}

	// Should never reach here, but return empty result just in case
	return TestResult{StepName: step.Name, Error: fmt.Errorf("unexpected error in retry logic")}
}

// executeStep performs the actual step execution
func (r *Runner) executeStep(ctx context.Context, gameStateID uuid.UUID, step TestStep, prevTurnCounter int, prevInventory []string, seedState *state.GameState) TestResult {
	start := time.Now()
	result := TestResult{
		StepName: step.Name,
	}

	// Check if this is a reset step
	if step.UserPrompt == ResetGameStatePrompt {
		err := r.resetGameState(ctx, gameStateID, seedState)
		if err != nil {
			result.Error = fmt.Errorf("failed to reset gamestate: %w", err)
			result.Duration = time.Since(start)
			return result
		}

		// For reset steps, we just check expectations against the reset state
		if step.Expectations.Location != nil || step.Expectations.SceneName != nil ||
			len(step.Expectations.Inventory) > 0 || len(step.Expectations.Vars) > 0 ||
			len(step.Expectations.NPCLocations) > 0 {

			resetState, err := r.getGameState(ctx, gameStateID)
			if err != nil {
				result.Error = fmt.Errorf("failed to get reset gamestate for expectations: %w", err)
				result.Duration = time.Since(start)
				return result
			}

			// Check expectations against reset state (use seedState as "previous" state)
			if err := r.checkExpectations(step.Expectations, seedState, resetState, prevTurnCounter, prevInventory, ""); err != nil {
				result.Error = fmt.Errorf("reset expectation failed: %w", err)
				result.Duration = time.Since(start)
				return result
			}
		}

		result.Success = true
		result.IsReset = true
		result.ResponseText = "[GAMESTATE RESET]"
		result.Duration = time.Since(start)
		return result
	}

	// Get gamestate before chat for expectations comparison
	preGameState, err := r.getGameState(ctx, gameStateID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get gamestate before chat: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	initialHistoryLen := len(preGameState.ChatHistory)

	// Post async chat message and get request_id
	requestID, err := PostChatAsync(ctx, r.Client, r.BaseURL, gameStateID, step.UserPrompt)
	if err != nil {
		result.Error = fmt.Errorf("failed to post async chat: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	result.RequestID = requestID

	// Poll for chat response (wait for history to increase)
	afterChatState, assistantResponse, err := PollForChatResponse(ctx, r.Client, r.BaseURL, gameStateID, initialHistoryLen)
	if err != nil {
		result.Error = fmt.Errorf("failed to poll for chat response: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	result.ResponseText = assistantResponse

	// Poll for DeltaWorker completion (wait for meta fields to update)
	postGameState, err := PollForDeltaWorkerCompletion(ctx, r.Client, r.BaseURL, gameStateID, afterChatState)
	if err != nil {
		result.Error = fmt.Errorf("failed to poll for DeltaWorker completion: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Check expectations
	if err := r.checkExpectations(step.Expectations, preGameState, postGameState, prevTurnCounter, prevInventory, assistantResponse); err != nil {
		result.Error = fmt.Errorf("expectation failed: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Success = true
	result.Duration = time.Since(start)
	return result
}

// getGameState retrieves the current gamestate
func (r *Runner) getGameState(ctx context.Context, gameStateID uuid.UUID) (*state.GameState, error) {
	return GetGameState(ctx, r.Client, r.BaseURL, gameStateID)
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

	// Full inventory check (order independent)
	if len(exp.Inventory) > 0 {
		// Create maps for efficient comparison
		expected := make(map[string]bool)
		for _, item := range exp.Inventory {
			expected[item] = true
		}

		actual := make(map[string]bool)
		for _, item := range postState.Inventory {
			actual[item] = true
		}

		// Check for missing items
		for expectedItem := range expected {
			if !actual[expectedItem] {
				return fmt.Errorf("expected inventory to contain '%s', but it's missing. Actual inventory: %v", expectedItem, postState.Inventory)
			}
		}

		// Check for extra items
		for actualItem := range actual {
			if !expected[actualItem] {
				return fmt.Errorf("inventory contains unexpected item '%s'. Expected inventory: %v, Actual: %v", actualItem, exp.Inventory, postState.Inventory)
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

	if exp.TurnCounter != nil {
		if postState.TurnCounter != *exp.TurnCounter {
			return fmt.Errorf("expected turn_counter to be %d, got %d", *exp.TurnCounter, postState.TurnCounter)
		}
	}

	if exp.SceneTurnCounter != nil {
		if postState.SceneTurnCounter != *exp.SceneTurnCounter {
			return fmt.Errorf("expected scene_turn_counter to be %d, got %d", *exp.SceneTurnCounter, postState.SceneTurnCounter)
		}
	}

	if exp.IsEnded != nil {
		if postState.IsEnded != *exp.IsEnded {
			return fmt.Errorf("expected is_ended to be %t, got %t", *exp.IsEnded, postState.IsEnded)
		}
	}

	return nil
}
