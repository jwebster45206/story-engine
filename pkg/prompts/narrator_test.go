package prompts

import (
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/pkg/character"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestBuildNarratorMessages_RequiresGameState(t *testing.T) {
	_, err := BuildNarratorMessages(nil, &scenario.Scenario{Name: "Test"}, "hi", 10, "")
	if err == nil {
		t.Fatal("expected error when gamestate is not set")
	}
	if err.Error() != "gamestate is required" {
		t.Errorf("got %v", err)
	}
}

func TestBuildNarratorMessages_RequiresScenario(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	_, err := BuildNarratorMessages(gs, nil, "hi", 10, "")
	if err == nil {
		t.Fatal("expected error when scenario is not set")
	}
	if err.Error() != "scenario is required" {
		t.Errorf("got %v", err)
	}
}

func TestBuildNarratorMessages_BasicMessages(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	sc := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {Name: "start", Description: "Starting location"},
		},
	}

	messages, err := BuildNarratorMessages(gs, sc, "Hello world", 20, "")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}
	if messages[0].Role != chat.ChatRoleSystem {
		t.Errorf("first message role = %s, want system", messages[0].Role)
	}
	if !messages[0].IsPersistent {
		t.Error("first system message should be persistent")
	}
	if len(messages) < 3 {
		t.Fatalf("expected at least 3 messages (2 system + user), got %d", len(messages))
	}
	if messages[1].Role != chat.ChatRoleSystem {
		t.Errorf("second message role = %s, want system", messages[1].Role)
	}
	if messages[1].IsPersistent {
		t.Error("second system message should not be persistent")
	}
	user := messages[len(messages)-1]
	if user.Role != chat.ChatRoleUser {
		t.Errorf("last message role = %s, want user", user.Role)
	}
	if !strings.HasPrefix(user.Content, "Hello world") {
		t.Errorf("user message = %q", user.Content)
	}
	if !strings.Contains(user.Content, "<rules>") {
		t.Error("expected <rules> block")
	}
}

func TestBuildNarratorMessages_WithNarrator(t *testing.T) {
	narrator := &scenario.Narrator{
		ID:      "test_narrator",
		Name:    "Test Narrator",
		Prompts: []string{"You are a test narrator.", "Speak in a test voice."},
	}
	gs := state.NewGameState("test.json", narrator, "test-provider", "test-model")
	gs.Location = "start"
	sc := basicScenario()

	messages, err := BuildNarratorMessages(gs, sc, "Test", 20, "")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	if !strings.Contains(messages[0].Content, "You are a test narrator.") {
		t.Error("expected system prompt to contain narrator prompts")
	}
}

func TestBuildNarratorMessages_WithPC(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.PC = &character.PC{
		ID:          "test_pc",
		Name:        "Test Character",
		Description: "A brave adventurer",
		HP:          10,
		MaxHP:       10,
		ContingencyPrompts: []conditionals.ContingencyPrompt{
			{Prompt: "You are playing as a test character."},
		},
	}
	gs.Location = "start"
	sc := basicScenario()

	messages, err := BuildNarratorMessages(gs, sc, "Test", 20, "")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	if !strings.Contains(messages[0].Content, "Test Character") {
		t.Error("expected persistent prompt to contain PC name")
	}
	if !strings.Contains(messages[1].Content, "You are playing as a test character.") {
		t.Error("expected PC contingency prompts in the dynamic block")
	}
}

func TestBuildNarratorMessages_WithChatHistory(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	gs.ChatHistory = []chat.ChatMessage{
		{Role: chat.ChatRoleUser, Content: "Message 1"},
		{Role: chat.ChatRoleAgent, Content: "Response 1"},
		{Role: chat.ChatRoleUser, Content: "Message 2"},
		{Role: chat.ChatRoleAgent, Content: "Response 2"},
	}

	messages, err := BuildNarratorMessages(gs, basicScenario(), "Message 3", 20, "")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	if len(messages) != 7 {
		t.Errorf("expected 7 messages (2 system + 4 history + 1 user), got %d", len(messages))
	}
	if messages[2].Content != "Message 1" {
		t.Error("expected chat history to be included")
	}
}

func TestBuildNarratorMessages_HistoryWindowing(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	for range 15 {
		gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
			Role:    chat.ChatRoleUser,
			Content: "Message",
		})
	}

	messages, err := BuildNarratorMessages(gs, basicScenario(), "Test", 5, "")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	if len(messages) != 8 {
		t.Errorf("expected 8 messages (2 system + 5 history + 1 user), got %d", len(messages))
	}
}

func TestBuildNarratorMessages_GameEnded(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	gs.IsEnded = true
	sc := basicScenario()
	sc.GameEndPrompt = "The adventure has ended!"

	messages, err := BuildNarratorMessages(gs, sc, "Test", 20, "")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	last := messages[len(messages)-1]
	if !strings.Contains(last.Content, "The adventure has ended!") {
		t.Error("expected final message to contain game end prompt")
	}
}

func TestBuildNarratorMessages_WithContingencyPrompts(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	gs.Vars = map[string]string{"test_flag": "true"}
	sc := basicScenario()
	sc.ContingencyPrompts = []conditionals.ContingencyPrompt{
		{Prompt: "Always show this prompt"},
		{
			Prompt: "Show when flag is true",
			When:   &conditionals.ConditionalWhen{Vars: map[string]string{"test_flag": "true"}},
		},
	}

	messages, err := BuildNarratorMessages(gs, sc, "Test", 20, "")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	dynamic := messages[1].Content
	if !strings.Contains(dynamic, "Always show this prompt") {
		t.Error("expected contingency prompts")
	}
	if !strings.Contains(dynamic, "Show when flag is true") {
		t.Error("expected conditional contingency prompts")
	}
}

func TestBuildNarratorMessages_RelaxedSystemPrompt(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Rules = state.RulesRelaxed
	sc := &scenario.Scenario{Name: "Test", Story: "A test story", Rating: scenario.RatingPG}

	messages, err := BuildNarratorMessages(gs, sc, "look around", 20, "")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	persistent := messages[0].Content
	if !strings.Contains(persistent, "You may extend it with plausible architecture") {
		t.Error("expected relaxed location language")
	}
	if strings.Contains(persistent, "Do not allow the user to control NPCs, create NPCs, invent items") {
		t.Error("strict invention ban should not appear in relaxed mode")
	}
	if strings.Contains(persistent, "HOW YOU INTERPRET USER PROMPTS") {
		t.Error("narrator should not interpret/allow user actions")
	}
	user := messages[len(messages)-1].Content
	if !strings.Contains(user, "<rules>") {
		t.Error("expected <rules> block on user message")
	}
	if !strings.Contains(user, "Do not act or speak for the Player Character") {
		t.Error("expected narratorRules in user message")
	}
}

func TestBuildNarratorMessages_StrictAndRelaxedShareRulesBlock(t *testing.T) {
	sc := &scenario.Scenario{Name: "Test", Story: "Story", Rating: scenario.RatingPG}
	strictGS := state.NewGameState("test.json", nil, "test-provider", "test-model")
	strictGS.Rules = state.RulesStrict
	relaxedGS := state.NewGameState("test.json", nil, "test-provider", "test-model")
	relaxedGS.Rules = state.RulesRelaxed

	strictMsgs, err := BuildNarratorMessages(strictGS, sc, "hi", 20, "")
	if err != nil {
		t.Fatalf("strict: %v", err)
	}
	relaxedMsgs, err := BuildNarratorMessages(relaxedGS, sc, "hi", 20, "")
	if err != nil {
		t.Fatalf("relaxed: %v", err)
	}

	strictRules := strictMsgs[len(strictMsgs)-1].Content[strings.Index(strictMsgs[len(strictMsgs)-1].Content, "<rules>"):]
	relaxedRules := relaxedMsgs[len(relaxedMsgs)-1].Content[strings.Index(relaxedMsgs[len(relaxedMsgs)-1].Content, "<rules>"):]
	if strictRules != relaxedRules {
		t.Errorf("narratorRules block should be identical;\nstrict:\n%s\nrelaxed:\n%s", strictRules, relaxedRules)
	}
}

func TestBuildNarratorMessages_RefereeAfterRules(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	sc := basicScenario()

	messages, err := BuildNarratorMessages(gs, sc, "I walk north", 20, "- Attempted: walk north\n- Not allowed: no such exit")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	user := messages[len(messages)-1].Content
	rulesIdx := strings.Index(user, "<rules>")
	refIdx := strings.Index(user, "<referee>")
	if rulesIdx < 0 || refIdx < 0 {
		t.Fatalf("expected both blocks, got %q", user)
	}
	if refIdx < rulesIdx {
		t.Fatal("referee block should follow <rules>")
	}
	if !strings.Contains(user, refereeHonorLine) {
		t.Error("expected honor line after referee block")
	}
	if !strings.HasPrefix(user, "I walk north") {
		t.Errorf("user text should come first, got %q", user)
	}
}

func TestBuildNarratorMessages_EmptyRefereeOmitsBlock(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	sc := &scenario.Scenario{Name: "Test", Story: "Story", Rating: scenario.RatingPG}

	messages, err := BuildNarratorMessages(gs, sc, "hi", 20, "  ")
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	user := messages[len(messages)-1].Content
	if strings.Contains(user, "<referee>") {
		t.Errorf("empty referee should omit the block, got %q", user)
	}
	if !strings.Contains(user, "<rules>") {
		t.Error("rules block should still be present")
	}
}

func TestBuildNarratorMessages_PersistentCacheSafety(t *testing.T) {
	gs := state.NewGameState("test.json", &scenario.Narrator{
		Name:    "Classic",
		Prompts: []string{"Speak plainly."},
	}, "test-provider", "test-model")
	gs.PC = &character.PC{ID: "hero", Name: "Hero", Description: "A traveler"}
	gs.Location = "start"
	gs.WorldLocations = map[string]scenario.Location{
		"start": {Name: "Start", Description: "A clearing.", Exits: map[string]string{"east": "cave"}},
		"cave":  {Name: "Cave", Description: "A dark cave.", Preview: "A dark cave mouth."},
	}
	sc := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": gs.WorldLocations["start"],
			"cave":  gs.WorldLocations["cave"],
		},
		ContingencyPrompts: []conditionals.ContingencyPrompt{
			{Prompt: "Always show this prompt"},
			{
				Prompt: "Show when flag is true",
				When:   &conditionals.ConditionalWhen{Vars: map[string]string{"flag": "true"}},
			},
		},
	}

	first, err := BuildNarratorMessages(gs, sc, "look around", 20, "")
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	persistent := first[0].Content

	turns := []func(){
		func() {
			gs.JustEntered = true
			gs.Inventory = []string{"torch"}
			gs.Vars = map[string]string{"flag": "true"}
		},
		func() {
			gs.Location = "cave"
			gs.JustEntered = true
			gs.NPCs = map[string]character.NPC{
				"guide": {Name: "Guide", Location: "cave"},
			}
		},
		func() {
			gs.JustEntered = false
			gs.SceneTurnCounter = 3
			gs.ChatHistory = []chat.ChatMessage{
				{Role: chat.ChatRoleUser, Content: "I enter the cave"},
				{Role: chat.ChatRoleAgent, Content: "The cave is cold."},
			}
		},
		func() {
			gs.Inventory = []string{"torch", "map"}
			gs.Vars["flag"] = "false"
		},
	}

	for i, mutate := range turns {
		mutate()
		msgs, err := BuildNarratorMessages(gs, sc, "continue", 20, "Allowed.")
		if err != nil {
			t.Fatalf("turn %d: %v", i+2, err)
		}
		if msgs[0].Content != persistent {
			t.Fatalf("persistent block drifted on turn %d\n--- first ---\n%s\n--- later ---\n%s", i+2, persistent, msgs[0].Content)
		}
		if !msgs[0].IsPersistent || msgs[1].IsPersistent {
			t.Fatalf("turn %d persistence flags = %v, %v", i+2, msgs[0].IsPersistent, msgs[1].IsPersistent)
		}
	}
}

func basicScenario() *scenario.Scenario {
	return &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {Name: "start", Description: "Starting location"},
		},
	}
}
