package state

import (
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/queue"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

// MergeConditionals evaluates conditionals once and merges triggered conditionals into the delta
// Also handles prompt-based actions (story events, etc.) by queuing them
// Returns a map of triggered conditional IDs to their conditionals for logging purposes
// Note: This only evaluates conditionals ONCE. For cascading conditionals, the caller should
// call this method repeatedly after applying the delta to game state.
func (a *Applier) MergeConditionals() map[string]scenario.Conditional {
	if a.scenario == nil {
		return nil
	}

	triggeredConditionals := a.scenario.EvaluateConditionals(a.gs)
	if len(triggeredConditionals) == 0 {
		return nil
	}

	triggered := make(map[string]scenario.Conditional)
	for conditionalID, conditional := range triggeredConditionals {
		triggered[conditionalID] = conditional
		// Merge into the existing delta
		a.mergeDelta(&conditional.Then, conditionalID)
	}

	return triggered
}

// mergeDelta merges a conditional's delta into the worker's delta, with special handling for prompts
func (a *Applier) mergeDelta(conditionalDelta *conditionals.GameStateDelta, conditionalID string) {
	if conditionalDelta == nil {
		return
	}

	// Merge scene change
	if conditionalDelta.SceneChange != nil && conditionalDelta.SceneChange.To != "" {
		a.delta.SceneChange = &struct {
			To     string `json:"to"`
			Reason string `json:"reason"`
		}{
			To:     conditionalDelta.SceneChange.To,
			Reason: "conditional",
		}
	}

	// Merge game ended state, overriding any previous value
	if conditionalDelta.GameEnded != nil {
		a.delta.GameEnded = conditionalDelta.GameEnded
	}

	// Merge user location, overriding any previous value
	if conditionalDelta.UserLocation != "" {
		a.delta.UserLocation = conditionalDelta.UserLocation
	}

	// Merge variables, overriding any previous values
	if len(conditionalDelta.SetVars) > 0 {
		if a.delta.SetVars == nil {
			a.delta.SetVars = make(map[string]string)
		}
		maps.Copy(a.delta.SetVars, conditionalDelta.SetVars)
	}

	// Merge item events
	if len(conditionalDelta.ItemEvents) > 0 {
		a.delta.ItemEvents = append(a.delta.ItemEvents, conditionalDelta.ItemEvents...)
	}

	// Merge NPC events
	if len(conditionalDelta.NPCEvents) > 0 {
		a.delta.NPCEvents = append(a.delta.NPCEvents, conditionalDelta.NPCEvents...)
	}

	// Merge monster events
	if len(conditionalDelta.MonsterEvents) > 0 {
		a.delta.MonsterEvents = append(a.delta.MonsterEvents, conditionalDelta.MonsterEvents...)
	}

	// Handle prompt - any prompt in a conditional is treated as a story event
	if conditionalDelta.Prompt != nil {
		prompt := *conditionalDelta.Prompt
		// Check if this story event has already fired
		if !a.hasStoryEventFired(conditionalID) {
			// Queue the story event
			a.queueStoryEvent(conditionalID, prompt)
		} else if a.logger != nil {
			a.logger.Debug("Story event already fired, skipping",
				"game_state_id", a.gs.ID.String(),
				"conditional_id", conditionalID)
		}
	}
}

// hasStoryEventFired checks if a story event has already been fired
func (a *Applier) hasStoryEventFired(conditionalID string) bool {
	if a.gs == nil || a.gs.FiredStoryEvents == nil {
		return false
	}
	return slices.Contains(a.gs.FiredStoryEvents, conditionalID)
}

// queueStoryEvent queues a single story event for the next turn and marks it as fired
func (a *Applier) queueStoryEvent(conditionalID string, eventText string) {
	// Queue service is required for story events
	if a.queue == nil {
		if a.logger != nil {
			a.logger.Error("No queue service configured, story event will be lost",
				"game_state_id", a.gs.ID.String(),
				"event", eventText)
		}
		return
	}

	req := &queue.Request{
		RequestID:   uuid.New().String(),
		Type:        queue.RequestTypeStoryEvent,
		GameStateID: a.gs.ID,
		EventPrompt: eventText,
		EnqueuedAt:  time.Now(),
	}

	if err := a.queue.EnqueueRequest(a.ctx, req); err != nil {
		if a.logger != nil {
			a.logger.Error("Failed to enqueue story event to unified queue",
				"error", err,
				"game_state_id", a.gs.ID.String(),
				"request_id", req.RequestID,
				"event", eventText)
		}
	} else {
		// Successfully queued - mark this story event as fired
		if a.gs.FiredStoryEvents == nil {
			a.gs.FiredStoryEvents = make([]string, 0)
		}
		a.gs.FiredStoryEvents = append(a.gs.FiredStoryEvents, conditionalID)

		if a.logger != nil {
			a.logger.Info("Story event enqueued to unified queue",
				"game_state_id", a.gs.ID.String(),
				"request_id", req.RequestID,
				"conditional_id", conditionalID,
				"event_prompt", eventText)
		}
	}
}
