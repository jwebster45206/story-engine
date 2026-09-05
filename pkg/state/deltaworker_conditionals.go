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
func (dw *DeltaWorker) MergeConditionals() map[string]scenario.Conditional {
	if dw.scenario == nil {
		return nil
	}

	triggeredConditionals := dw.scenario.EvaluateConditionals(dw.gs)
	if len(triggeredConditionals) == 0 {
		return nil
	}

	triggered := make(map[string]scenario.Conditional)
	for conditionalID, conditional := range triggeredConditionals {
		triggered[conditionalID] = conditional
		// Merge into the existing delta
		dw.mergeDelta(&conditional.Then, conditionalID)
	}

	return triggered
}

// mergeDelta merges a conditional's delta into the worker's delta, with special handling for prompts
func (dw *DeltaWorker) mergeDelta(conditionalDelta *conditionals.GameStateDelta, conditionalID string) {
	if conditionalDelta == nil {
		return
	}

	// Merge scene change
	if conditionalDelta.SceneChange != nil && conditionalDelta.SceneChange.To != "" {
		dw.delta.SceneChange = &struct {
			To     string `json:"to"`
			Reason string `json:"reason"`
		}{
			To:     conditionalDelta.SceneChange.To,
			Reason: "conditional",
		}
	}

	// Merge game ended state, overriding any previous value
	if conditionalDelta.GameEnded != nil {
		dw.delta.GameEnded = conditionalDelta.GameEnded
	}

	// Merge user location, overriding any previous value
	if conditionalDelta.UserLocation != "" {
		dw.delta.UserLocation = conditionalDelta.UserLocation
	}

	// Merge variables, overriding any previous values
	if len(conditionalDelta.SetVars) > 0 {
		if dw.delta.SetVars == nil {
			dw.delta.SetVars = make(map[string]string)
		}
		maps.Copy(dw.delta.SetVars, conditionalDelta.SetVars)
	}

	// Merge item events
	if len(conditionalDelta.ItemEvents) > 0 {
		dw.delta.ItemEvents = append(dw.delta.ItemEvents, conditionalDelta.ItemEvents...)
	}

	// Merge NPC events
	if len(conditionalDelta.NPCEvents) > 0 {
		dw.delta.NPCEvents = append(dw.delta.NPCEvents, conditionalDelta.NPCEvents...)
	}

	// Merge monster events
	if len(conditionalDelta.MonsterEvents) > 0 {
		dw.delta.MonsterEvents = append(dw.delta.MonsterEvents, conditionalDelta.MonsterEvents...)
	}

	// Handle prompt - any prompt in a conditional is treated as a story event
	if conditionalDelta.Prompt != nil {
		prompt := *conditionalDelta.Prompt
		// Check if this story event has already fired
		if !dw.hasStoryEventFired(conditionalID) {
			// Queue the story event
			dw.queueStoryEvent(conditionalID, prompt)
		} else if dw.logger != nil {
			dw.logger.Debug("Story event already fired, skipping",
				"game_state_id", dw.gs.ID.String(),
				"conditional_id", conditionalID)
		}
	}
}

// hasStoryEventFired checks if a story event has already been fired
func (dw *DeltaWorker) hasStoryEventFired(conditionalID string) bool {
	if dw.gs == nil || dw.gs.FiredStoryEvents == nil {
		return false
	}
	return slices.Contains(dw.gs.FiredStoryEvents, conditionalID)
}

// queueStoryEvent queues a single story event for the next turn and marks it as fired
func (dw *DeltaWorker) queueStoryEvent(conditionalID string, eventText string) {
	// Queue service is required for story events
	if dw.queue == nil {
		if dw.logger != nil {
			dw.logger.Error("No queue service configured, story event will be lost",
				"game_state_id", dw.gs.ID.String(),
				"event", eventText)
		}
		return
	}

	req := &queue.Request{
		RequestID:   uuid.New().String(),
		Type:        queue.RequestTypeStoryEvent,
		GameStateID: dw.gs.ID,
		EventPrompt: eventText,
		EnqueuedAt:  time.Now(),
	}

	if err := dw.queue.EnqueueRequest(dw.ctx, req); err != nil {
		if dw.logger != nil {
			dw.logger.Error("Failed to enqueue story event to unified queue",
				"error", err,
				"game_state_id", dw.gs.ID.String(),
				"request_id", req.RequestID,
				"event", eventText)
		}
	} else {
		// Successfully queued - mark this story event as fired
		if dw.gs.FiredStoryEvents == nil {
			dw.gs.FiredStoryEvents = make([]string, 0)
		}
		dw.gs.FiredStoryEvents = append(dw.gs.FiredStoryEvents, conditionalID)

		if dw.logger != nil {
			dw.logger.Info("Story event enqueued to unified queue",
				"game_state_id", dw.gs.ID.String(),
				"request_id", req.RequestID,
				"conditional_id", conditionalID,
				"event_prompt", eventText)
		}
	}
}
