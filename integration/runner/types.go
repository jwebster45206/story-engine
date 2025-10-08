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
type TestSuite struct {
	Name          string          `yaml:"name"`
	Scenario      string          `yaml:"scenario"`
	SeedGameState state.GameState `yaml:"seed_game_state"`
	Steps         []TestStep      `yaml:"steps"`
}

// TestStep defines a single test interaction and its expected outcomes
// Use user_prompt: "RESET_GAMESTATE" to reset to the original seed state
type TestStep struct {
	Name         string       `yaml:"name,omitempty"`
	UserPrompt   string       `yaml:"user_prompt"`
	Expectations Expectations `yaml:"expect"`
}

// Expectations defines what to check after a test step executes
type Expectations struct {
	// GameState properties - aligned with pkg/state/gamestate.go
	Location         *string           `yaml:"location,omitempty"`           // User location
	SceneName        *string           `yaml:"scene_name,omitempty"`         // Current scene name
	Inventory        []string          `yaml:"inventory,omitempty"`          // Full inventory contents (order independent)
	TurnCounter      *int              `yaml:"turn_counter,omitempty"`       // Total turn count
	SceneTurnCounter *int              `yaml:"scene_turn_counter,omitempty"` // Scene-specific turn count
	IsEnded          *bool             `yaml:"is_ended,omitempty"`           // Game ended state
	Vars             map[string]string `yaml:"vars,omitempty"`               // Game variables
	// NPC Locations (check specific NPC locations)
	NPCLocations map[string]string `yaml:"npc_locations,omitempty"`

	// Response Analysis
	ResponseContains    []string `yaml:"response_contains,omitempty"`
	ResponseNotContains []string `yaml:"response_not_contains,omitempty"`
	ResponseRegex       string   `yaml:"response_regex,omitempty"`
	ResponseMinLength   *int     `yaml:"response_min_length,omitempty"`
	ResponseMaxLength   *int     `yaml:"response_max_length,omitempty"`
}

// TestResult contains the outcome of running a test step
type TestResult struct {
	TestName     string
	StepName     string
	Success      bool
	Error        error
	Duration     time.Duration
	ResponseText string
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
