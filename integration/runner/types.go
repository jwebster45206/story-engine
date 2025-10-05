package runner

import (
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// TestSuite defines a complete integration test scenario
type TestSuite struct {
	Name          string          `yaml:"name"`
	Scenario      string          `yaml:"scenario"`
	SeedGameState state.GameState `yaml:"seed_game_state"`
	Steps         []TestStep      `yaml:"steps"`
}

// TestStep defines a single test interaction and its expected outcomes
type TestStep struct {
	Name         string       `yaml:"name,omitempty"`
	UserPrompt   string       `yaml:"user_prompt"`
	Expectations Expectations `yaml:"expect"`
}

// Expectations defines what to check after a test step executes
type Expectations struct {
	// Location & Scene
	Location       *string `yaml:"location,omitempty"`
	SceneName      *string `yaml:"scene_name,omitempty"`
	SceneChange    *string `yaml:"scene_change,omitempty"`    // expect scene to change to this value
	SceneUnchanged *string `yaml:"scene_unchanged,omitempty"` // expect scene to remain this value

	// Inventory Changes (check for additions/removals)
	InventoryAdded   []string `yaml:"inventory_added,omitempty"`
	InventoryRemoved []string `yaml:"inventory_removed,omitempty"`

	// Variables (check only specified keys)
	Vars map[string]string `yaml:"vars,omitempty"`

	// NPC Locations (check specific NPC locations)
	NPCLocations map[string]string `yaml:"npc_locations,omitempty"`

	// Response Analysis
	ResponseContains    []string `yaml:"response_contains,omitempty"`
	ResponseNotContains []string `yaml:"response_not_contains,omitempty"`
	ResponseRegex       string   `yaml:"response_regex,omitempty"`
	ResponseMinLength   *int     `yaml:"response_min_length,omitempty"`
	ResponseMaxLength   *int     `yaml:"response_max_length,omitempty"`

	// Game Flow
	GameEnded     *bool `yaml:"game_ended,omitempty"`
	TurnIncrement *int  `yaml:"turn_increment,omitempty"` // relative to previous step
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
