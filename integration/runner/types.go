package runner

import (
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// Special user prompt values that trigger non-chat actions
const (
	ResetGameStatePrompt = "RESET_GAMESTATE"
)

// TestSuite defines a complete integration test scenario
// Can either be a regular test with Steps, or a suite that references other Cases
type TestSuite struct {
	Name          string          `json:"name"`
	Scenario      string          `json:"scenario,omitempty"`        // Used for regular tests
	SeedGameState state.GameState `json:"seed_game_state,omitempty"` // Used for regular tests
	Steps         []TestStep      `json:"steps,omitempty"`           // Used for regular tests
	Cases         []string        `json:"cases,omitempty"`           // Used for suite tests (list of case files)
}

// IsSequence returns true if this is a suite that sequences other cases
func (ts *TestSuite) IsSequence() bool {
	return len(ts.Cases) > 0
}

// TestStep defines a single test interaction and its expected outcomes
// Use user_prompt: "RESET_GAMESTATE" to reset to the original seed state
type TestStep struct {
	Name         string       `json:"name,omitempty"`
	UserPrompt   string       `json:"user_prompt"`
	Expectations Expectations `json:"expect"`
}

// Expectations defines what to check after a test step executes
type Expectations struct {
	// GameState properties - aligned with pkg/state/gamestate.go
	Location         *string           `json:"location,omitempty"`           // User location
	SceneName        *string           `json:"scene_name,omitempty"`         // Current scene name
	Inventory        []string          `json:"inventory,omitempty"`          // Full inventory contents (order independent)
	TurnCounter      *int              `json:"turn_counter,omitempty"`       // Total turn count
	SceneTurnCounter *int              `json:"scene_turn_counter,omitempty"` // Scene-specific turn count
	IsEnded          *bool             `json:"is_ended,omitempty"`           // Game ended state
	Vars             map[string]string `json:"vars,omitempty"`               // Game variables
	// NPC Locations (check specific NPC locations)
	NPCLocations map[string]string `json:"npc_locations,omitempty"`

	// Response Analysis
	ResponseContains    []string `json:"response_contains,omitempty"`
	ResponseNotContains []string `json:"response_not_contains,omitempty"`
	ResponseRegex       string   `json:"response_regex,omitempty"`
	ResponseMinLength   *int     `json:"response_min_length,omitempty"`
	ResponseMaxLength   *int     `json:"response_max_length,omitempty"`
}

// TestResult contains the outcome of running a test step
type TestResult struct {
	TestName     string
	StepName     string
	Success      bool
	Error        error
	Duration     time.Duration
	ResponseText string
	IsReset      bool // True if this was a RESET_GAMESTATE step (should not count toward pass/fail metrics)
}

// TestJob represents a test suite to be executed by a worker
type TestJob struct {
	Name     string
	Suite    TestSuite
	CaseFile string
}

// TestRunResult contains the results of running an entire test suite
type TestRunResult struct {
	Job       TestJob
	Results   []TestResult
	Error     error
	Duration  time.Duration
	GameState uuid.UUID // ID of the gamestate used for this test
}
