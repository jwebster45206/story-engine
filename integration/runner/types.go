package runner

import (
	"time"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/state"
)

// Special user prompt values that trigger non-chat actions
const (
	ResetGameStatePrompt    = "RESET_GAMESTATE"
	WaitForStoryEventPrompt = "WAIT_FOR_STORY_EVENT"
)

// TestSuite defines a complete integration test scenario.
type TestSuite struct {
	Name          string          `json:"name"`
	Scenario      string          `json:"scenario,omitempty"`
	SeedGameState state.GameState `json:"seed_game_state,omitzero"`
	Steps         []TestStep      `json:"steps,omitempty"`
}

// TestStep defines a single test interaction and its expected outcomes
// Use user_prompt: "RESET_GAMESTATE" to reset to the original seed state
// Use user_prompt: "WAIT_FOR_STORY_EVENT" to wait for a story event to trigger and complete
type TestStep struct {
	Name         string       `json:"name,omitempty"`
	UserPrompt   string       `json:"user_prompt"`
	Expectations Expectations `json:"expect"`
}

// Expectations defines what to check after a test step executes.
// JSON names match gamestate fields. Turn counters are not assertable.
type Expectations struct {
	Location             *string           `json:"user_location,omitempty"`
	SceneName            *string           `json:"scene_name,omitempty"`
	InventoryContains    []string          `json:"inventory_contains,omitempty"`
	InventoryNotContains []string          `json:"inventory_not_contains,omitempty"`
	IsEnded              *bool             `json:"is_ended,omitempty"`
	Vars                 map[string]string `json:"vars,omitempty"`
	NPCLocations         map[string]string `json:"npc_locations,omitempty"`

	// CombatRoll requires a combat-scoped attempt on the player turn and
	// another on the strike-back story event. Hit, miss, and HP are ignored.
	CombatRoll bool `json:"combat_roll,omitempty"`

	ResponseContains    []string `json:"response_contains,omitempty"`
	ResponseNotContains []string `json:"response_not_contains,omitempty"`
	ResponseRegex       string   `json:"response_regex,omitempty"`
	ResponseMinLength   *int     `json:"response_min_length,omitempty"`
	ResponseMaxLength   *int     `json:"response_max_length,omitempty"`

	StoryEventContains    []string `json:"story_event_contains,omitempty"`
	StoryEventNotContains []string `json:"story_event_not_contains,omitempty"`
	StoryEventExact       *string  `json:"story_event_exact,omitempty"`
}

// TestResult contains the outcome of running a test step
type TestResult struct {
	TestName         string
	StepName         string
	RequestID        string // Request ID from async chat endpoint
	Success          bool
	Error            error
	Duration         time.Duration
	ResponseText     string
	StoryEventText   string // Story event message text (for WAIT_FOR_STORY_EVENT steps)
	IsReset          bool   // True if this was a RESET_GAMESTATE step (should not count toward pass/fail metrics)
	IsStoryEventWait bool   // True if this was a WAIT_FOR_STORY_EVENT step
}

// TestJob represents a test suite to be executed by a worker
type TestJob struct {
	Name  string
	Suite TestSuite
}

// TestRunResult contains the results of running an entire test suite
type TestRunResult struct {
	Job       TestJob
	Results   []TestResult
	Error     error
	Duration  time.Duration
	GameState uuid.UUID // ID of the gamestate used for this test
}
