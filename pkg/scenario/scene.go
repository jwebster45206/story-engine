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
	StoryEvents        map[string]StoryEvent            `json:"story_events,omitempty"` // Priority narrative events with conditions (key = event ID)
}

// Conditional represents a deterministic rule to execute when conditions are met
type Conditional struct {
	When conditionals.ConditionalWhen `json:"when"` // Conditions that must be met
	Then ConditionalThen              `json:"then"` // Actions to execute when conditions are met
}

// ConditionalThen defines the actions to take when conditions are met
type ConditionalThen struct {
	Scene     string `json:"scene,omitempty"`      // Change to this scene
	GameEnded *bool  `json:"game_ended,omitempty"` // Set game ended state (true/false)
	// TODO: Add inventory modifications
	// AddItems    []string `json:"add_items,omitempty"`
	// RemoveItems []string `json:"remove_items,omitempty"`
}

// StoryEvent represents a priority narrative event that gets injected into the story flow
type StoryEvent struct {
	When   conditionals.ConditionalWhen `json:"when"`   // Conditions that must be met for event to trigger
	Prompt string                       `json:"prompt"` // The narrative text to inject
}
