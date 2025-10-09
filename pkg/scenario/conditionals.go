package scenario

// GameStateView provides the minimal interface needed to evaluate conditionals
// This avoids an import cycle with the state package
type GameStateView interface {
	GetSceneName() string
	GetVars() map[string]string
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
	if len(when.Vars) == 0 && len(when.Counters) == 0 && when.Location == "" {
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

	// TODO: Check counter conditions
	// TODO: Check location condition

	// All conditions passed
	return true
}
