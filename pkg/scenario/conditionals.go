package scenario

import "github.com/jwebster45206/story-engine/pkg/conditionals"

// EvaluateConditionals checks all conditionals for the current scene and returns triggered conditionals
// Returns a map of conditional IDs to their conditionals
func (s *Scenario) EvaluateConditionals(gsView conditionals.GameStateView) map[string]Conditional {
	sceneName := gsView.GetSceneName()
	if sceneName == "" {
		return nil
	}

	scene, exists := s.Scenes[sceneName]
	if !exists || len(scene.Conditionals) == 0 {
		return nil
	}

	triggered := make(map[string]Conditional)

	for conditionalID, conditional := range scene.Conditionals {
		if conditionals.EvaluateWhen(conditional.When, gsView) {
			triggered[conditionalID] = conditional
		}
	}

	if len(triggered) == 0 {
		return nil
	}

	return triggered
}

// EvaluateStoryEvents checks all story events for the current scene and returns triggered events
// Returns a map of event IDs to their story events
func (s *Scenario) EvaluateStoryEvents(gsView conditionals.GameStateView) map[string]StoryEvent {
	sceneName := gsView.GetSceneName()
	if sceneName == "" {
		return nil
	}

	scene, exists := s.Scenes[sceneName]
	if !exists || len(scene.StoryEvents) == 0 {
		return nil
	}

	triggered := make(map[string]StoryEvent)

	for eventKey, event := range scene.StoryEvents {
		if conditionals.EvaluateWhen(event.When, gsView) {
			triggered[eventKey] = event
		}
	}

	if len(triggered) == 0 {
		return nil
	}

	return triggered
}

// FilterContingencyPrompts returns only the prompts whose conditions are met
// Prompts without conditions (When == nil) are always included
func FilterContingencyPrompts(prompts []conditionals.ContingencyPrompt, gsView conditionals.GameStateView) []string {
	return conditionals.FilterContingencyPrompts(prompts, gsView)
}
