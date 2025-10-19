package conditionals

import "encoding/json"

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

// ConditionalWhen defines the conditions that must be met for a conditional to trigger
type ConditionalWhen struct {
	Vars             map[string]string `json:"vars,omitempty"`               // All specified variables must match
	SceneTurnCounter *int              `json:"scene_turn_counter,omitempty"` // Exact match for scene turn counter
	TurnCounter      *int              `json:"turn_counter,omitempty"`       // Exact match for turn counter
	Location         string            `json:"location,omitempty"`           // User must be at this location
	MinSceneTurns    *int              `json:"min_scene_turns,omitempty"`    // Scene turn counter >= this value
	MinTurns         *int              `json:"min_turns,omitempty"`          // Turn counter >= this value
}

// GameStateView provides the minimal interface needed to evaluate conditionals
// This avoids import cycles with the state package
type GameStateView interface {
	GetSceneName() string
	GetVars() map[string]string
	GetSceneTurnCounter() int
	GetTurnCounter() int
	GetUserLocation() string
}

// FilterContingencyPrompts returns only the prompts whose conditions are met
// Prompts without conditions (When == nil) are always included
func FilterContingencyPrompts(prompts []ContingencyPrompt, gsView GameStateView) []string {
	var active []string
	for _, cp := range prompts {
		// If no conditions, always include
		if cp.When == nil {
			active = append(active, cp.Prompt)
			continue
		}

		// Check if conditions are met
		if EvaluateWhen(*cp.When, gsView) {
			active = append(active, cp.Prompt)
		}
	}
	return active
}

// EvaluateWhen checks if all conditions in a When clause are met
func EvaluateWhen(when ConditionalWhen, gsView GameStateView) bool {
	// If no conditions specified, return false (conditional should not trigger)
	hasCondition := len(when.Vars) > 0 ||
		when.SceneTurnCounter != nil ||
		when.TurnCounter != nil ||
		when.Location != "" ||
		when.MinSceneTurns != nil ||
		when.MinTurns != nil

	if !hasCondition {
		return false
	}

	// Check variable conditions
	if len(when.Vars) > 0 {
		gameVars := gsView.GetVars()
		if gameVars == nil {
			return false
		}

		for varName, expectedValue := range when.Vars {
			actualValue, exists := gameVars[varName]
			if !exists || actualValue != expectedValue {
				return false
			}
		}
	}

	// Check scene turn counter (exact match)
	if when.SceneTurnCounter != nil {
		if gsView.GetSceneTurnCounter() != *when.SceneTurnCounter {
			return false
		}
	}

	// Check turn counter (exact match)
	if when.TurnCounter != nil {
		if gsView.GetTurnCounter() != *when.TurnCounter {
			return false
		}
	}

	// Check scene turn counter minimum
	if when.MinSceneTurns != nil {
		if gsView.GetSceneTurnCounter() < *when.MinSceneTurns {
			return false
		}
	}

	// Check turn counter minimum
	if when.MinTurns != nil {
		if gsView.GetTurnCounter() < *when.MinTurns {
			return false
		}
	}

	// Check location condition
	if when.Location != "" {
		if gsView.GetUserLocation() != when.Location {
			return false
		}
	}

	// All conditions passed
	return true
}
