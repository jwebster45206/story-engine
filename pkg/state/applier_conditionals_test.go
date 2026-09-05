package state

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/queue"
	"github.com/jwebster45206/story-engine/pkg/scenario"
)

type recordingQueue struct {
	n int
}

func (q *recordingQueue) GetFormattedEvents(context.Context, uuid.UUID) (string, error) {
	return "", nil
}

func (q *recordingQueue) Clear(context.Context, uuid.UUID) error { return nil }

func (q *recordingQueue) EnqueueRequest(context.Context, *queue.Request) error {
	q.n++
	return nil
}

func TestApplier_StoryEventFiresOnce(t *testing.T) {
	prompt := "STORY EVENT: Dracula appears."
	gs := &GameState{
		ID:        uuid.New(),
		SceneName: "test",
		Vars:      map[string]string{"trigger": "true"},
	}
	scen := &scenario.Scenario{
		Scenes: map[string]scenario.Scene{
			"test": {
				Conditionals: map[string]scenario.Conditional{
					"dracula_appears": {
						When: conditionals.ConditionalWhen{Vars: map[string]string{"trigger": "true"}},
						Then: conditionals.GameStateDelta{Prompt: &prompt},
					},
				},
			},
		},
	}
	q := &recordingQueue{}
	a := NewApplier(gs, &conditionals.GameStateDelta{}, scen, nil).WithQueue(q)

	a.MergeConditionals()
	if q.n != 1 {
		t.Fatalf("first merge enqueued %d, want 1", q.n)
	}
	if len(gs.FiredStoryEvents) != 1 || gs.FiredStoryEvents[0] != "dracula_appears" {
		t.Fatalf("FiredStoryEvents = %v, want [dracula_appears]", gs.FiredStoryEvents)
	}

	a.MergeConditionals()
	if q.n != 1 {
		t.Fatalf("second merge enqueued again, n=%d", q.n)
	}
}
