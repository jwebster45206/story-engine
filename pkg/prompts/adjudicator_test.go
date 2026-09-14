package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestAdjudicatorWindow(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, AdjudicatorHistoryLimit},
		{-1, AdjudicatorHistoryLimit},
		{2, 2},
		{4, 4},
		{16, AdjudicatorHistoryLimit},
	}
	for _, tt := range tests {
		if got := AdjudicatorWindow(tt.in); got != tt.want {
			t.Errorf("AdjudicatorWindow(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestBuildAdjudicatorMessages_RequiresGameState(t *testing.T) {
	_, err := BuildAdjudicatorMessages(nil, "hi", 4)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildAdjudicatorMessages_StrictVsRelaxed(t *testing.T) {
	strictGS := adjudicatorTestGS(state.RulesStrict)
	relaxedGS := adjudicatorTestGS(state.RulesRelaxed)

	strictMsgs, err := BuildAdjudicatorMessages(strictGS, "I walk north", 4)
	if err != nil {
		t.Fatalf("strict: %v", err)
	}
	relaxedMsgs, err := BuildAdjudicatorMessages(relaxedGS, "I walk north", 4)
	if err != nil {
		t.Fatalf("relaxed: %v", err)
	}

	strictSys := strictMsgs[0].Content
	relaxedSys := relaxedMsgs[0].Content

	if !strings.Contains(strictSys, AdjudicatorPrompt) {
		t.Error("expected adjudicator preamble")
	}
	if !strings.Contains(strictSys, "<world_state>") {
		t.Error("expected world_state")
	}
	if !strings.Contains(strictSys, "The Tavern") {
		t.Error("expected current location in world_state")
	}
	if strings.Contains(strictSys, "Content Rating") {
		t.Error("adjudicator should not include content rating")
	}
	if strings.Contains(strictSys, "You are a test narrator") {
		t.Error("adjudicator should not include narrator style")
	}
	if !strings.Contains(strictSys, "Do not allow the user to control NPCs, create NPCs, invent items") {
		t.Error("expected strict interpretation")
	}
	if strings.Contains(strictSys, "Honor people, creatures, places, and objects the player introduces") {
		t.Error("strict messages should not include relaxed interpretation")
	}
	if !strings.Contains(relaxedSys, "Honor people, creatures, places, and objects the player introduces") {
		t.Error("expected relaxed interpretation")
	}

	user := strictMsgs[len(strictMsgs)-1]
	if user.Role != chat.ChatRoleUser || user.Content != "I walk north" {
		t.Errorf("user = %+v", user)
	}
	if strings.Contains(user.Content, "<rules>") {
		t.Error("adjudicator user turn should not include narrator <rules>")
	}
}

func TestBuildAdjudicatorMessages_HistoryWindow(t *testing.T) {
	gs := adjudicatorTestGS(state.RulesStrict)
	gs.ChatHistory = make([]chat.ChatMessage, 8)
	for i := range gs.ChatHistory {
		gs.ChatHistory[i] = chat.ChatMessage{Role: chat.ChatRoleUser, Content: "drop"}
	}
	gs.ChatHistory[6].Content = "keep-a"
	gs.ChatHistory[7].Content = "keep-b"

	msgs, err := BuildAdjudicatorMessages(gs, "now", 2)
	if err != nil {
		t.Fatalf("BuildAdjudicatorMessages: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("len = %d, want 4", len(msgs))
	}
	if msgs[1].Content != "keep-a" || msgs[2].Content != "keep-b" {
		t.Errorf("history window = %q, %q", msgs[1].Content, msgs[2].Content)
	}
}

func adjudicatorTestGS(mode state.RulesMode) *state.GameState {
	gs := state.NewGameState("test.json", &scenario.Narrator{
		Name:    "Test Narrator",
		Prompts: []string{"You are a test narrator"},
	}, "test-provider", "test-model")
	gs.Rules = mode
	gs.Location = "tavern"
	gs.WorldLocations = map[string]scenario.Location{
		"tavern": {
			Name:        "The Tavern",
			Description: "A smoky taproom.",
			Exits:       map[string]string{"north": "street"},
		},
	}
	return gs
}
