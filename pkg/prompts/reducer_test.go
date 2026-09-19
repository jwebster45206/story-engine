package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestBuildReducerMessages_RequiresInputs(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	sc := &scenario.Scenario{Name: "Test"}

	if _, err := BuildReducerMessages(nil, sc, "reply"); err == nil {
		t.Fatal("expected error when gamestate is not set")
	}
	if _, err := BuildReducerMessages(gs, nil, "reply"); err == nil {
		t.Fatal("expected error when scenario is not set")
	}
}

func TestBuildReducerMessages_PersistentThenDynamic(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	gs.SceneName = "dock"
	gs.WorldLocations = map[string]scenario.Location{
		"start": {Name: "Start", Description: "A pier."},
	}
	sc := &scenario.Scenario{
		Name:             "Test",
		ContingencyRules: []string{"Scenario rule."},
		Scenes: map[string]scenario.Scene{
			"dock": {ContingencyRules: []string{"When the shipwright starts repairs, the scene changes to british_docks."}},
		},
	}

	msgs, err := BuildReducerMessages(gs, sc, "You take the repair ledger.")
	if err != nil {
		t.Fatalf("BuildReducerMessages: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("len = %d, want 4", len(msgs))
	}
	if msgs[0].Role != chat.ChatRoleSystem || !msgs[0].IsPersistent {
		t.Fatalf("first = %#v, want persistent system", msgs[0])
	}
	if msgs[0].Content != ReducerPrompt {
		t.Fatal("persistent block should be the reducer instruction body")
	}
	if strings.Contains(msgs[0].Content, "CONTINGENCY RULES") {
		t.Fatal("persistent block should not include contingency rules")
	}
	if strings.Contains(msgs[0].Content, "BEFORE game state") {
		t.Fatal("persistent block should not include BEFORE state")
	}

	dynamic := msgs[1]
	if dynamic.Role != chat.ChatRoleSystem || dynamic.IsPersistent {
		t.Fatalf("second = %#v, want dynamic system", dynamic)
	}
	if strings.Contains(dynamic.Content, "CONTINGENCY RULES") {
		t.Fatal("dynamic block should omit contingency rules")
	}
	if strings.Contains(dynamic.Content, "Scenario rule.") {
		t.Fatal("dynamic block should omit scenario rules")
	}
	if strings.Contains(dynamic.Content, "When the shipwright starts repairs") {
		t.Fatal("dynamic block should omit scene rules")
	}
	if !strings.Contains(dynamic.Content, "BEFORE game state:") {
		t.Fatal("dynamic block should include BEFORE state")
	}
	if !strings.Contains(dynamic.Content, `"user_location":"start"`) && !strings.Contains(dynamic.Content, `"user_location": "start"`) {
		t.Fatalf("dynamic block should include location, got %s", dynamic.Content)
	}

	if msgs[2].Role != chat.ChatRoleAgent || msgs[2].Content != "You take the repair ledger." {
		t.Fatalf("agent = %#v", msgs[2])
	}
	if msgs[3].Role != chat.ChatRoleUser || msgs[3].Content != "Interpret the game state delta from assistant's last message." {
		t.Fatalf("extract = %#v", msgs[3])
	}
	for i, m := range msgs[:3] {
		if m.Role == chat.ChatRoleUser {
			t.Fatalf("player prompt should be omitted, msgs[%d] = %#v", i, m)
		}
	}
}

func TestBuildReducerMessages_PersistentCacheSafety(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	gs.SceneName = "dock"
	gs.WorldLocations = map[string]scenario.Location{
		"start": {Name: "Start", Description: "A pier.", Exits: map[string]string{"east": "cave"}},
		"cave":  {Name: "Cave", Description: "A dark cave."},
	}
	sc := &scenario.Scenario{
		Name: "Test",
		Scenes: map[string]scenario.Scene{
			"dock":    {ContingencyRules: []string{"Dock rule."}},
			"repairs": {ContingencyRules: []string{"Repairs rule."}},
		},
	}

	first, err := BuildReducerMessages(gs, sc, "The pier is quiet.")
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	persistent := first[0].Content

	gs.Location = "cave"
	gs.SceneName = "repairs"
	gs.Inventory = []string{"ledger"}
	gs.Vars = map[string]string{"shipwright_hired": "true"}
	later, err := BuildReducerMessages(gs, sc, "You enter the cave.")
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	if later[0].Content != persistent {
		t.Fatal("persistent reducer block drifted after state/scene change")
	}
	if strings.Contains(later[1].Content, "Repairs rule.") || strings.Contains(later[1].Content, "Dock rule.") {
		t.Fatal("dynamic block should not include scene rules")
	}
	if !strings.Contains(later[1].Content, "BEFORE game state:") {
		t.Fatal("dynamic block should include BEFORE state")
	}
}
