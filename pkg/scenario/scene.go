package scenario

// Scene represents a single scene within a scenario with its own locations, NPCs, and rules
type Scene struct {
	Story              string              `json:"story"`                  // Description of what happens in this scene
	Locations          map[string]Location `json:"locations"`              // Map of location names to Location objects for this scene
	NPCs               map[string]NPC      `json:"npcs"`                   // Map of NPC names to their data for this scene
	Vars               map[string]string   `json:"vars"`                   // Scene-specific variables
	ContingencyPrompts []string            `json:"contingency_prompts"`    // Conditional prompts for LLM in this scene
	ContingencyRules   []string            `json:"contingency_rules"`      // Backend rules for LLM to follow in this scene
	Conditionals       []Conditional       `json:"conditionals,omitempty"` // Deterministic when/then rules
}

// Conditional represents a deterministic rule to execute when conditions are met
type Conditional struct {
	Name string          `json:"name,omitempty"` // Optional name for debugging
	When ConditionalWhen `json:"when"`           // Conditions that must be met
	Then ConditionalThen `json:"then"`           // Actions to execute when conditions are met
}

// ConditionalWhen defines the conditions that must be met for a conditional to trigger
type ConditionalWhen struct {
	Vars     map[string]string `json:"vars,omitempty"`     // Variable conditions (all must match)
	Counters map[string]int    `json:"counters,omitempty"` // Counter conditions (all must match) - TODO
	Location string            `json:"location,omitempty"` // Location condition - TODO
}

// ConditionalThen defines the actions to take when conditions are met
type ConditionalThen struct {
	Scene     string `json:"scene,omitempty"`      // Change to this scene
	GameEnded *bool  `json:"game_ended,omitempty"` // Set game ended state (true/false)
	// TODO: Add inventory modifications
	// AddItems    []string `json:"add_items,omitempty"`
	// RemoveItems []string `json:"remove_items,omitempty"`
}
