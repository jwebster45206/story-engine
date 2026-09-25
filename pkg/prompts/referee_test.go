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
		{2, refereeHistoryLimit},
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
	if !strings.Contains(strictSys, "subject:") {
		t.Error("expected subject field in preamble")
	}
	if !strings.Contains(strictSys, "object:") {
		t.Error("expected object field in preamble")
	}
	if strings.Contains(strictSys, "focus:") {
		t.Error("focus should not be in the referee preamble")
	}
	if !strings.Contains(strictSys, `"NPCs here"`) || !strings.Contains(strictSys, `"Monsters here"`) {
		t.Error("object preamble should name NPCs here and Monsters here")
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
	if !strings.Contains(strictSys, `"object":"drawbridge"`) {
		t.Error("strict examples should include movement object")
	}
	if !strings.Contains(strictSys, `"object":"Giant Rat"`) {
		t.Error("strict examples should include a combat object")
	}
	if !strings.Contains(strictSys, `"subject":null`) {
		t.Error("strict examples should omit PC as subject")
	}
	if strings.Contains(strictSys, `"focus"`) {
		t.Error("strict examples should not include focus")
	}
	if strings.Contains(strictSys, `"npcs"`) {
		t.Error("strict examples should not use focus.npcs")
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
	if !strings.Contains(strictSys, "The player roleplays as the Player Character") {
		t.Error("expected strict PC-only global rule")
	}
	if !strings.Contains(strictSys, `A "Name: " prefix on the user line is the player when Name is the PC`) {
		t.Error("expected strict PC name-prefix rule")
	}
	if strings.Contains(strictSys, "Player Character (PC):") {
		t.Error("PC identity line should be omitted when gs.PC is nil")
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
	if len(msgs) != 3 {
		t.Fatalf("len = %d, want 3", len(msgs))
	}
	if msgs[1].Content != "keep-b" {
		t.Errorf("history window = %q, want keep-b", msgs[1].Content)
	}
	if msgs[2].Content != "now" {
		t.Errorf("current user = %q, want now", msgs[2].Content)
	}
}

func TestBuildRefereeMessages_NamesPC(t *testing.T) {
	gs := refereeTestGS(state.RulesStrict)
	gs.PC = &character.PC{Name: "Captain Jack Sparrow"}

	msgs, err := BuildRefereeMessages(gs, "I slash the skeleton.", 1)
	if err != nil {
		t.Fatalf("BuildRefereeMessages: %v", err)
	}
	sys := msgs[0].Content
	want := "Player Character (PC): Captain Jack Sparrow. User lines prefixed with that name are the player acting as the PC, not an NPC."
	if !strings.Contains(sys, want) {
		t.Errorf("system prompt missing PC identity, got %q", sys)
	}
	worldIdx := strings.Index(sys, "<world_state>")
	pcIdx := strings.Index(sys, "Player Character (PC):")
	if worldIdx < 0 || pcIdx < 0 || pcIdx > worldIdx {
		t.Error("PC identity should appear before WORLD STATE")
	}
}

func TestBuildRefereeMessages_AfterStrikeBack(t *testing.T) {
	gs := refereeTestGS(state.RulesStrict)
	gs.PC = &character.PC{Name: "Captain Jack Sparrow"}
	gs.ChatHistory = []chat.ChatMessage{
		{Role: chat.ChatRoleAgent, Content: "You are Captain Jack Sparrow."},
		{Role: chat.ChatRoleUser, Content: "Captain Jack Sparrow: I slash the skeleton."},
		{Role: chat.ChatRoleAgent, Content: "You crack a rib off the skeleton."},
		{Role: chat.ChatRoleUser, Content: "Skeleton strikes Captain Jack Sparrow.", IsStoryEvent: true},
		{Role: chat.ChatRoleAgent, Content: "The skeleton's bony knuckles catch you square across the jaw."},
	}
	current := "Captain Jack Sparrow: I hit it again."

	msgs, err := BuildRefereeMessages(gs, current, 16)
	if err != nil {
		t.Fatalf("BuildRefereeMessages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("len = %d, want 3 (system, last assistant, current user)", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "Player Character (PC): Captain Jack Sparrow") {
		t.Error("expected named PC in system prompt")
	}
	if msgs[1].Role != chat.ChatRoleAgent || !strings.Contains(msgs[1].Content, "bony knuckles") {
		t.Errorf("history = %+v, want last assistant narration", msgs[1])
	}
	for _, m := range msgs {
		if strings.Contains(m.Content, "Skeleton strikes Captain Jack Sparrow.") {
			t.Fatalf("story-event user line should not be in referee messages: %+v", m)
		}
	}
	if msgs[2].Role != chat.ChatRoleUser || msgs[2].Content != current {
		t.Errorf("current user = %+v", msgs[2])
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
	gs.NPCs = map[string]*character.NPC{
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
