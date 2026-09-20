package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestRefereeWindow(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, refereeHistoryLimit},
		{-1, refereeHistoryLimit},
		{1, 1},
		{2, 2},
		{4, refereeHistoryLimit},
		{16, refereeHistoryLimit},
	}
	for _, tt := range tests {
		if got := refereeWindow(tt.in); got != tt.want {
			t.Errorf("refereeWindow(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFormatRefereeBlock(t *testing.T) {
	if got := formatRefereeBlock(); got != "" {
		t.Errorf("empty args = %q, want empty", got)
	}
	if got := formatRefereeBlock("  ", ""); got != "" {
		t.Errorf("blank parts = %q, want empty", got)
	}
	got := formatRefereeBlock("Allowed.", "Felix strikes Giant Rat and hits.", "Giant Rat strikes Felix and misses.")
	want := "<referee>\nAllowed.\nFelix strikes Giant Rat and hits.\nGiant Rat strikes Felix and misses.\n</referee>\n" + refereeHonorLine
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildRefereeMessages_RequiresGameState(t *testing.T) {
	_, err := BuildRefereeMessages(nil, "hi", 2)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRefereeMessages_StrictVsRelaxed(t *testing.T) {
	strictGS := refereeTestGS(state.RulesStrict)
	relaxedGS := refereeTestGS(state.RulesRelaxed)

	strictMsgs, err := BuildRefereeMessages(strictGS, "I walk north", 2)
	if err != nil {
		t.Fatalf("strict: %v", err)
	}
	relaxedMsgs, err := BuildRefereeMessages(relaxedGS, "I walk north", 2)
	if err != nil {
		t.Fatalf("relaxed: %v", err)
	}

	strictSys := strictMsgs[0].Content
	relaxedSys := relaxedMsgs[0].Content

	if !strings.Contains(strictSys, refereePrompt) {
		t.Error("expected referee preamble")
	}
	if strings.Contains(strictSys, "up to two sentences") {
		t.Error("preamble should not require two-sentence prose")
	}
	if !strings.Contains(strictSys, "allowed:") {
		t.Error("expected allowed field in preamble")
	}
	if !strings.Contains(strictSys, "reaction:") {
		t.Error("expected reaction field in preamble")
	}
	if !strings.Contains(strictSys, "scope:") {
		t.Error("expected scope field in preamble")
	}
	if !strings.Contains(strictSys, "focus:") {
		t.Error("expected focus field in preamble")
	}
	if !strings.Contains(strictSys, `"NPCs here"`) || !strings.Contains(strictSys, `"Monsters here"`) {
		t.Error("focus preamble should name NPCs here and Monsters here")
	}
	if strings.Contains(strictSys, "mentioned:") {
		t.Error("mentioned should not be in the referee preamble")
	}
	if !strings.Contains(strictSys, "dialogue, movement, combat, examine, ambient, other") {
		t.Error("expected scope enum in preamble")
	}
	if !strings.Contains(strictSys, `"allowed":true`) && !strings.Contains(strictSys, `"allowed":false`) {
		t.Error("strict examples should contain allowed JSON")
	}
	if !strings.Contains(strictSys, `"scope":"movement"`) {
		t.Error("strict examples should include scope")
	}
	if !strings.Contains(strictSys, `"locations":["drawbridge"]`) {
		t.Error("strict examples should include focus.locations")
	}
	if !strings.Contains(strictSys, `"actors":["Giant Rat"]`) {
		t.Error("strict examples should include a combat actor")
	}
	if strings.Contains(strictSys, `"reasoning":"The PC`) {
		t.Error("strict example reasoning should be second person, not The PC")
	}
	if !strings.Contains(strictSys, `"reasoning":"You cannot move to the banquet hall`) {
		t.Error("strict examples should include second-person deny reasoning")
	}
	if strings.Contains(strictSys, `"npcs"`) {
		t.Error("strict examples should use focus.actors, not focus.npcs")
	}
	if strings.Contains(strictSys, `"items"`) {
		t.Error("strict examples should not include focus.items")
	}
	for _, heading := range []string{"Movement\n-", "Items\n-", "NPCs\n-", "Global\n-"} {
		if !strings.Contains(strictSys, heading) {
			t.Errorf("strict prompt missing heading %q", heading)
		}
		if !strings.Contains(relaxedSys, heading) {
			t.Errorf("relaxed prompt missing heading %q", heading)
		}
	}
	if !strings.Contains(strictSys, "The player may only travel listed exits") {
		t.Error("expected strict movement rule")
	}
	if strings.Contains(strictSys, "Known exits are the obvious paths") {
		t.Error("strict messages should not include relaxed movement")
	}
	if !strings.Contains(relaxedSys, "Known exits are the obvious paths") {
		t.Error("expected relaxed movement rule")
	}
	if strings.Contains(relaxedSys, "The player may only travel listed exits") {
		t.Error("relaxed messages should not include strict movement")
	}
	if strings.Contains(strictSys, "Honor people, creatures, places, and objects the player introduces") {
		t.Error("strict messages should not include relaxed global rule")
	}
	if !strings.Contains(relaxedSys, "Honor people, creatures, places, and objects the player introduces") {
		t.Error("expected relaxed global rule")
	}

	examplesIdx := strings.Index(strictSys, "Examples:")
	worldEnd := strings.Index(strictSys, "</world_state>")
	if examplesIdx < 0 {
		t.Error("strict prompt should include examples")
	} else if worldEnd < 0 || examplesIdx < worldEnd {
		t.Error("strict examples should follow world_state")
	}
	if strings.Contains(relaxedSys, "Examples:") {
		t.Error("relaxed prompt should not include examples")
	}

	if strings.Contains(strictSys, "### Describing locations") {
		t.Error("referee should not include narrator Describing locations")
	}
	if strings.Contains(strictSys, "weave real exits") {
		t.Error("referee should not include narrator location prose")
	}
	if strings.Contains(strictSys, "Content Rating") {
		t.Error("referee should not include content rating")
	}
	if strings.Contains(strictSys, "You are a test narrator") {
		t.Error("referee should not include narrator style")
	}
	if strings.Contains(strictSys, "<just_entered>") {
		t.Error("referee should not include just_entered")
	}
	if strings.Contains(strictSys, "<world_state_rules>") {
		t.Error("referee should not include world_state_rules")
	}

	if !strings.Contains(strictSys, "<world_state>") {
		t.Error("expected world_state")
	}
	if !strings.Contains(strictSys, "The Tavern") {
		t.Error("expected current location in world_state")
	}
	if !strings.Contains(strictSys, "A smoky taproom.") {
		t.Error("expected location description")
	}
	if !strings.Contains(strictSys, "<adjacent_previews>") {
		t.Error("expected adjacent_previews")
	}
	if !strings.Contains(strictSys, "north -> Harbor Street") {
		t.Error("expected listed exit")
	}
	if !strings.Contains(strictSys, "Items here: mug") {
		t.Error("expected location items")
	}
	if !strings.Contains(strictSys, "NPCs here: Bartender") {
		t.Error("expected NPCs here")
	}
	if !strings.Contains(strictSys, "torch") {
		t.Error("expected inventory")
	}

	user := strictMsgs[len(strictMsgs)-1]
	if user.Role != chat.ChatRoleUser || user.Content != "I walk north" {
		t.Errorf("user = %+v", user)
	}
	if strings.Contains(user.Content, "<rules>") {
		t.Error("referee user turn should not include narrator <rules>")
	}
}

func TestBuildRefereeMessages_HistoryWindow(t *testing.T) {
	gs := refereeTestGS(state.RulesStrict)
	gs.ChatHistory = make([]chat.ChatMessage, 8)
	for i := range gs.ChatHistory {
		gs.ChatHistory[i] = chat.ChatMessage{Role: chat.ChatRoleUser, Content: "drop"}
	}
	gs.ChatHistory[6].Content = "keep-a"
	gs.ChatHistory[7].Content = "keep-b"

	msgs, err := BuildRefereeMessages(gs, "now", 2)
	if err != nil {
		t.Fatalf("BuildRefereeMessages: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("len = %d, want 4", len(msgs))
	}
	if msgs[1].Content != "keep-a" || msgs[2].Content != "keep-b" {
		t.Errorf("history window = %q, %q", msgs[1].Content, msgs[2].Content)
	}
}

func refereeTestGS(mode state.RulesMode) *state.GameState {
	gs := state.NewGameState("test.json", &scenario.Narrator{
		Name:    "Test Narrator",
		Prompts: []string{"You are a test narrator"},
	}, "test-provider", "test-model")
	gs.Rules = mode
	gs.Location = "tavern"
	gs.Inventory = []string{"torch"}
	gs.NPCs = map[string]character.NPC{
		"bartender": {Name: "Bartender", Location: "tavern"},
	}
	gs.WorldLocations = map[string]scenario.Location{
		"tavern": {
			Name:        "The Tavern",
			Description: "A smoky taproom.",
			Items:       []string{"mug"},
			Exits:       map[string]string{"north": "street"},
		},
		"street": {
			Name:    "Harbor Street",
			Preview: "A cobblestone street.",
		},
	}
	return gs
}
