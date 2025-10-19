package scenario

import "encoding/json"

// Scene represents a single scene within a scenario with its own locations, NPCs, and rules
type Scene struct {
	Story              string                 `json:"story"`                  // Description of what happens in this scene
	Locations          map[string]Location    `json:"locations"`              // Map of location names to Location objects for this scene
	NPCs               map[string]NPC         `json:"npcs"`                   // Map of NPC names to their data for this scene
	Vars               map[string]string      `json:"vars"`                   // Scene-specific variables
	ContingencyPrompts []ContingencyPrompt    `json:"contingency_prompts"`    // Conditional prompts for LLM in this scene
	ContingencyRules   []string               `json:"contingency_rules"`      // Backend rules for LLM to follow in this scene
	Conditionals       map[string]Conditional `json:"conditionals,omitempty"` // Deterministic when/then rules (key = conditional ID)
	StoryEvents        map[string]StoryEvent  `json:"story_events,omitempty"` // Priority narrative events with conditions (key = event ID)
}

// ContingencyPrompt can be either a simple string (always shown) or a conditional prompt
type ContingencyPrompt struct {
	Prompt string           `json:"prompt"`         // The prompt text
	When   *ConditionalWhen `json:"when,omitempty"` // Optional conditions - if nil, always show
}

// UnmarshalJSON implements custom JSON unmarshaling to support both string and object formats
func (cp *ContingencyPrompt) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as a plain string first (backwards compatibility)
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		cp.Prompt = str
		cp.When = nil // No conditions means always show
		return nil
	}

	// Try unmarshaling as an object with conditions
	type Alias ContingencyPrompt
	aux := &struct{ *Alias }{Alias: (*Alias)(cp)}
	return json.Unmarshal(data, aux)
}

// Conditional represents a deterministic rule to execute when conditions are met
type Conditional struct {
	When ConditionalWhen `json:"when"` // Conditions that must be met
	Then ConditionalThen `json:"then"` // Actions to execute when conditions are met
}

// ConditionalWhen defines the conditions that must be met for a conditional to trigger
type ConditionalWhen struct {
	Vars             map[string]string `json:"vars,omitempty"`               // All specified variables must match
	SceneTurnCounter *int              `json:"scene_turn_counter,omitempty"` // Exact match for scene turn counter
	TurnCounter      *int              `json:"turn_counter,omitempty"`       // Exact match for turn counter
	Location         string            `json:"location,omitempty"`           // User must be at this location
	MinSceneTurns    *int              `json:"min_scene_turns,omitempty"`    // Scene turn counter >= this value
	MinTurns         *int              `json:"min_turns,omitempty"`          // Turn counter >= this value
} // ConditionalThen defines the actions to take when conditions are met
type ConditionalThen struct {
	Scene     string `json:"scene,omitempty"`      // Change to this scene
	GameEnded *bool  `json:"game_ended,omitempty"` // Set game ended state (true/false)
	// TODO: Add inventory modifications
	// AddItems    []string `json:"add_items,omitempty"`
	// RemoveItems []string `json:"remove_items,omitempty"`
}

// StoryEvent represents a priority narrative event that gets injected into the story flow
type StoryEvent struct {
	When   ConditionalWhen `json:"when"`   // Conditions that must be met for event to trigger
	Prompt string          `json:"prompt"` // The narrative text to inject
}
