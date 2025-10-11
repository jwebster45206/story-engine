package scenario

// GameStateView provides the minimal interface needed to evaluate conditionals
// This avoids an import cycle with the state package
type GameStateView interface {
	GetSceneName() string
	GetVars() map[string]string
	GetSceneTurnCounter() int
	GetTurnCounter() int
	GetUserLocation() string
}

// EvaluateConditionals checks all conditionals for the current scene and returns triggered conditionals
func (s *Scenario) EvaluateConditionals(gsView GameStateView) []Conditional {
	sceneName := gsView.GetSceneName()
	if sceneName == "" {
		return nil
	}

	scene, exists := s.Scenes[sceneName]
	if !exists || len(scene.Conditionals) == 0 {
		return nil
	}

	var triggered []Conditional

	for _, conditional := range scene.Conditionals {
		if evaluateWhen(conditional.When, gsView) {
			triggered = append(triggered, conditional)
		}
	}

	return triggered
}

// evaluateWhen checks if all conditions in a When clause are met
func evaluateWhen(when ConditionalWhen, gsView GameStateView) bool {
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
