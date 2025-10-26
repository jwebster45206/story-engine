package scenario

import (
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
)

const StoryEventPrefix = "STORY EVENT: "

// Scene represents a single scene within a scenario with its own locations, NPCs, and rules
type Scene struct {
	Story              string                           `json:"story"`                  // Description of what happens in this scene
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
