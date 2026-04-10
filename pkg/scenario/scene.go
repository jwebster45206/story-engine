package scenario

import (
	"strings"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

// StoryEventPrefix is the data-layer prefix used in scenario JSON files.
// The worker strips this before wrapping the content in XML plot directive tags for the LLM.
const StoryEventPrefix = "STORY EVENT: "

// PlotDirective XML tags used to wrap story events before sending to the LLM.
// Pseudo-XML tags are structurally distinct from prose and models avoid reproducing them.
const (
	PlotDirectiveOpen  = "<plot_directive>"
	PlotDirectiveClose = "</plot_directive>"
)

// FormatPlotDirective strips the data-layer "STORY EVENT: " prefix (if present)
// and wraps the content in <plot_directive> XML tags for LLM consumption.
func FormatPlotDirective(prompt string) string {
	content := strings.TrimPrefix(prompt, StoryEventPrefix)
	return PlotDirectiveOpen + content + PlotDirectiveClose
}

// Scene represents a single scene within a scenario with its own locations, NPCs, and rules
type Scene struct {
	Story              string                           `json:"story"`                  // Description of what happens in this scene
	Temperature        *float64                         `json:"temperature,omitempty"`  // LLM temperature override for this scene (0.0–1.0); overrides scenario-level setting
	Locations          map[string]Location              `json:"locations"`              // Map of location names to Location objects for this scene
	NPCs               map[string]actor.NPC             `json:"npcs"`                   // Map of NPC names to their data for this scene
	Vars               map[string]string                `json:"vars"`                   // Scene-specific variables
	ContingencyPrompts []conditionals.ContingencyPrompt `json:"contingency_prompts"`    // Conditional prompts for LLM in this scene
	ContingencyRules   []string                         `json:"contingency_rules"`      // Backend rules for LLM to follow in this scene
	Conditionals       map[string]Conditional           `json:"conditionals,omitempty"` // Deterministic when/then rules (key = conditional ID)
}

// Conditional represents a deterministic rule to execute when conditions are met
type Conditional struct {
	When conditionals.ConditionalWhen `json:"when"` // Conditions that must be met
	Then conditionals.GameStateDelta  `json:"then"` // Actions to execute when conditions are met
}
