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
	_, err := BuildNarratorMessages(nil, &scenario.Scenario{Name: "Test"}, "hi", 10, nil)
	if err == nil {
		t.Fatal("expected error when gamestate is not set")
	}
	if err.Error() != "gamestate is required" {
		t.Errorf("got %v", err)
	}
}

func TestBuildNarratorMessages_RequiresScenario(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	_, err := BuildNarratorMessages(gs, nil, "hi", 10, nil)
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

	messages, err := BuildNarratorMessages(gs, sc, "Hello world", 20, nil)
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

	messages, err := BuildNarratorMessages(gs, sc, "Test", 20, nil)
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

	messages, err := BuildNarratorMessages(gs, sc, "Test", 20, nil)
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

	messages, err := BuildNarratorMessages(gs, basicScenario(), "Message 3", 20, nil)
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

	messages, err := BuildNarratorMessages(gs, basicScenario(), "Test", 5, nil)
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

	messages, err := BuildNarratorMessages(gs, sc, "Test", 20, nil)
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
	gs.ChatHistory = []chat.ChatMessage{
		{Role: chat.ChatRoleAgent, Content: "A bell rings.", IsStoryEvent: true},
	}
	sc := basicScenario()
	sc.ContingencyPrompts = []conditionals.ContingencyPrompt{
		{Prompt: "Always show this prompt"},
		{
			Prompt: "Show when flag is true",
			When:   &conditionals.ConditionalWhen{Vars: map[string]string{"test_flag": "true"}},
		},
	}

	messages, err := BuildNarratorMessages(gs, sc, "Test", 20, nil)
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	dynamic := messages[1].Content
	if !strings.Contains(dynamic, "<contingencies>") {
		t.Error("expected contingencies wrapper")
	}
	if !strings.Contains(dynamic, "Always show this prompt") {
		t.Error("expected contingency prompts")
	}
	if !strings.Contains(dynamic, "Show when flag is true") {
		t.Error("expected conditional contingency prompts")
	}
	if !strings.HasSuffix(strings.TrimSpace(dynamic), "</contingencies>") {
		t.Error("contingencies should be last in the dynamic block")
	}
}

func TestBuildNarratorMessages_RelaxedSystemPrompt(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Rules = state.RulesRelaxed
	sc := &scenario.Scenario{Name: "Test", Story: "A test story", Rating: scenario.RatingPG}

	messages, err := BuildNarratorMessages(gs, sc, "look around", 20, nil)
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	persistent := messages[0].Content
	if !strings.Contains(persistent, "draw from the WORLD STATE as your starting point") {
		t.Error("expected relaxed location language")
	}
	if !strings.Contains(messages[1].Content, "The WORLD STATE is a starting point") {
		t.Error("expected relaxed world-state latitude")
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

	strictMsgs, err := BuildNarratorMessages(strictGS, sc, "hi", 20, nil)
	if err != nil {
		t.Fatalf("strict: %v", err)
	}
	relaxedMsgs, err := BuildNarratorMessages(relaxedGS, sc, "hi", 20, nil)
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

	messages, err := BuildNarratorMessages(gs, sc, "I walk north", 20, new(chat.Ruling{Allowed: false, Reasoning: "There is no north exit."}))
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
	if !strings.Contains(user, "Not allowed. There is no north exit.") {
		t.Errorf("expected ruling prose, got %q", user)
	}
	if !strings.HasPrefix(user, "I walk north") {
		t.Errorf("user text should come first, got %q", user)
	}
}

func TestBuildNarratorMessages_CombatStrikeLinesAfterRuling(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	sc := basicScenario()
	ruling := &chat.Ruling{Allowed: true, Reasoning: "The PC can strike the Giant Rat."}
	hit := "Felix strikes Giant Rat and hits."

	messages, err := BuildNarratorMessages(gs, sc, "I attack the rat", 20, ruling, hit)
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	user := messages[len(messages)-1].Content
	refIdx := strings.Index(user, "<referee>")
	endIdx := strings.Index(user, "</referee>")
	if refIdx < 0 || endIdx < 0 {
		t.Fatalf("expected referee block, got %q", user)
	}
	body := user[refIdx:endIdx]
	if !strings.Contains(body, "Allowed. The PC can strike the Giant Rat.") {
		t.Errorf("missing allow line in %q", body)
	}
	if !strings.Contains(body, hit) {
		t.Errorf("missing hit line in %q", body)
	}
	if strings.Contains(body, "Giant Rat strikes") {
		t.Errorf("should not include a reaction strike, got %q", body)
	}
	if strings.Contains(strings.ToLower(user), "rolled") || strings.Contains(user, "DC") {
		t.Errorf("narrator must not include dice mechanics, got %q", user)
	}
}

func TestBuildNarratorMessages_EmptyRefereeOmitsBlock(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	sc := &scenario.Scenario{Name: "Test", Story: "Story", Rating: scenario.RatingPG}

	messages, err := BuildNarratorMessages(gs, sc, "hi", 20, nil)
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

	first, err := BuildNarratorMessages(gs, sc, "look around", 20, nil)
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	persistent := first[0].Content
	if strings.Contains(persistent, "### Monsters") {
		t.Fatal("monsters copy must not live in the persistent block")
	}
	if !strings.Contains(persistent, "Content Rating: PG") {
		t.Fatal("rating must live in the persistent block")
	}
	if strings.Contains(first[1].Content, "Content Rating:") {
		t.Fatal("rating must not live in the dynamic block")
	}

	turns := []func(){
		func() {
			gs.Inventory = []string{"torch"}
			gs.Vars = map[string]string{"flag": "true"}
		},
		func() {
			gs.Location = "cave"
			gs.NPCs = map[string]*character.NPC{
				"guide": {Name: "Guide", Location: "cave"},
			}
		},
		func() {
			gs.SceneTurnCounter = 3
			gs.ChatHistory = []chat.ChatMessage{
				{Role: chat.ChatRoleUser, Content: "I enter the cave"},
				{Role: chat.ChatRoleAgent, Content: "The cave is cold."},
			}
		},
		func() {
			gs.Inventory = []string{"torch", "map"}
			gs.Vars["flag"] = "false"
			loc := gs.WorldLocations["cave"]
			loc.Monsters = map[string]*character.Monster{
				"rat1": {ID: "rat1", Name: "Giant Rat", AC: 12, HP: 7, MaxHP: 7},
			}
			gs.WorldLocations["cave"] = loc
		},
		func() {
			gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
				Role:         chat.ChatRoleAgent,
				Content:      "A bell tolls across the water.",
				IsStoryEvent: true,
			})
		},
	}

	for i, mutate := range turns {
		mutate()
		msgs, err := BuildNarratorMessages(gs, sc, "continue", 20, new(chat.Ruling{Allowed: true}))
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

func TestBuildNarratorMessages_PersistentDropsRedundantCopy(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	sc := basicScenario()

	messages, err := BuildNarratorMessages(gs, sc, "look around", 20, nil)
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	persistent := messages[0].Content
	for _, dropped := range []string{
		"you don't speak for the user",
		"Example (wrong)",
		"Example (right)",
		`Don't refer to "inventory"`,
		"gently remind them to stay in character",
		"Physical space first",
		"Source priority",
		"### Monsters",
		worldEventContinuationRule,
	} {
		if strings.Contains(persistent, dropped) {
			t.Errorf("persistent block still contains dropped copy %q", dropped)
		}
	}
	for _, kept := range []string{
		`Colons are reserved only for dialogue lines`,
		`CharacterName: "Spoken line here."`,
		"world's side of the conversation",
		"Do not break the fourth wall",
		"Do not answer questions about the game mechanics",
		"Move the story forward gradually",
		"### Describing locations",
		"Content Rating: PG",
	} {
		if !strings.Contains(persistent, kept) {
			t.Errorf("persistent block missing %q", kept)
		}
	}
	if strings.Contains(messages[1].Content, "Content Rating:") {
		t.Error("rating should not appear in the dynamic block")
	}
}

func TestBuildNarratorMessages_RulesBlockOwnsPCAndInvention(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	messages, err := BuildNarratorMessages(gs, basicScenario(), "hi", 20, nil)
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	user := messages[len(messages)-1].Content
	start := strings.Index(user, "<rules>")
	end := strings.Index(user, "</rules>")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("expected <rules> block, got %q", user)
	}
	rules := user[start : end+len("</rules>")]
	if n := strings.Count(rules, narratorRules[0]); n != 1 {
		t.Errorf("invention ban appears %d times in <rules>, want 1", n)
	}
	if n := strings.Count(rules, narratorRules[1]); n != 1 {
		t.Errorf("PC-ownership appears %d times in <rules>, want 1", n)
	}
}

func TestBuildNarratorMessages_MonstersCopyConditional(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	gs.WorldLocations = map[string]scenario.Location{
		"start": {Name: "Start", Description: "A clearing."},
	}
	sc := basicScenario()
	sc.Locations = gs.WorldLocations

	messages, err := BuildNarratorMessages(gs, sc, "look", 20, nil)
	if err != nil {
		t.Fatalf("no monsters: %v", err)
	}
	if strings.Contains(messages[0].Content, "### Monsters") {
		t.Error("monsters heading must not appear in persistent")
	}
	if strings.Contains(messages[1].Content, "### Monsters") {
		t.Error("monsters copy should be omitted when none are present")
	}

	loc := gs.WorldLocations["start"]
	loc.Monsters = map[string]*character.Monster{
		"rat1": {ID: "rat1", Name: "Giant Rat", AC: 12, HP: 7, MaxHP: 7},
	}
	gs.WorldLocations["start"] = loc

	messages, err = BuildNarratorMessages(gs, sc, "look", 20, nil)
	if err != nil {
		t.Fatalf("with monsters: %v", err)
	}
	if strings.Contains(messages[0].Content, "### Monsters") {
		t.Error("monsters heading must not appear in persistent")
	}
	dynamic := messages[1].Content
	if !strings.Contains(dynamic, "### Monsters") {
		t.Error("expected monsters copy when monsters are present")
	}
	if !strings.Contains(dynamic, strictMonsters) {
		t.Error("expected strict monsters copy in the dynamic block")
	}
}

func TestBuildNarratorMessages_MovementUsesDestAsCurrent(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	gs.WorldLocations = map[string]scenario.Location{
		"start": {
			Name:        "Forest Clearing",
			Description: "A quiet glade with ancient oaks.",
			Preview:     "A forest clearing.",
			Exits:       map[string]string{"east": "cave"},
		},
		"cave": {
			Name:        "Dark Cave",
			Description: "A dripping limestone cave.",
			Preview:     "A cave mouth.",
			Items:       []string{"rusty lantern"},
			Exits:       map[string]string{"west": "start"},
		},
	}
	gs.NPCs = map[string]*character.NPC{
		"hermit": {Name: "The Hermit", Location: "cave"},
	}
	sc := basicScenario()
	sc.Locations = gs.WorldLocations

	messages, err := BuildNarratorMessages(gs, sc, "look", 20, nil)
	if err != nil {
		t.Fatalf("nil ruling: %v", err)
	}
	dynamic := messages[1].Content
	if strings.Contains(dynamic, "<just_entered>") || strings.Contains(dynamic, "New location:") {
		t.Error("just_entered should be gone from the narrator prompt")
	}
	if strings.Contains(dynamic, "<destination>") {
		t.Error("destination sidecar should be omitted")
	}
	if strings.Contains(dynamic, "The PC is arriving") {
		t.Error("arrival directive should be omitted when not moving")
	}
	if current := currentLocationBlock(dynamic); !strings.Contains(current, "Forest Clearing") {
		t.Errorf("nil ruling should keep origin as current_location\n%s", current)
	}

	dialogue := new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeDialogue,
		Object:  "Guide",
	})
	messages, err = BuildNarratorMessages(gs, sc, "talk", 20, dialogue)
	if err != nil {
		t.Fatalf("dialogue: %v", err)
	}
	if strings.Contains(messages[1].Content, "The PC is arriving") {
		t.Error("dialogue should not include an arrival directive")
	}
	if current := currentLocationBlock(messages[1].Content); !strings.Contains(current, "Forest Clearing") {
		t.Errorf("dialogue should keep origin as current_location\n%s", current)
	}

	denied := new(chat.Ruling{
		Allowed:   false,
		Scope:     chat.RulingScopeMovement,
		Object:    "Dark Cave",
		Reasoning: "The way is blocked.",
	})
	messages, err = BuildNarratorMessages(gs, sc, "go east", 20, denied)
	if err != nil {
		t.Fatalf("denied move: %v", err)
	}
	dynamic = messages[1].Content
	if strings.Contains(dynamic, "<destination>") {
		t.Error("denied movement should not inject destination")
	}
	if strings.Contains(dynamic, "The PC is arriving") {
		t.Error("denied movement should not include an arrival directive")
	}
	if current := currentLocationBlock(dynamic); !strings.Contains(current, "Forest Clearing") {
		t.Errorf("denied movement should keep origin as current_location\n%s", current)
	}

	move := new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeMovement,
		Object:  "Dark Cave",
	})
	messages, err = BuildNarratorMessages(gs, sc, "go east", 20, move)
	if err != nil {
		t.Fatalf("allowed move: %v", err)
	}
	dynamic = messages[1].Content
	if gs.Location != "start" {
		t.Errorf("GameState.Location = %q, want start (prompt-only view)", gs.Location)
	}
	if strings.Contains(dynamic, "<destination>") {
		t.Error("destination sidecar should be gone; dest is current_location")
	}
	current := currentLocationBlock(dynamic)
	if !strings.Contains(current, "Dark Cave") {
		t.Errorf("current_location should be the destination\n%s", current)
	}
	if !strings.Contains(current, "A dripping limestone cave.") {
		t.Errorf("current_location should include dest description\n%s", current)
	}
	if !strings.Contains(current, "rusty lantern") {
		t.Errorf("current_location should include dest items\n%s", current)
	}
	if !strings.Contains(current, "The Hermit") {
		t.Errorf("current_location should include dest NPCs\n%s", current)
	}
	if strings.Contains(current, "A quiet glade with ancient oaks.") {
		t.Errorf("origin description must not be current_location\n%s", current)
	}

	empty := new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeMovement,
	})
	messages, err = BuildNarratorMessages(gs, sc, "go east", 20, empty)
	if err != nil {
		t.Fatalf("venice miss: %v", err)
	}
	dynamic = messages[1].Content
	if strings.Contains(dynamic, "<destination>") {
		t.Error("venice miss must not invent a destination block")
	}
	if strings.Contains(dynamic, "The PC is arriving") {
		t.Error("venice miss must not invent an arrival line")
	}
	if current := currentLocationBlock(dynamic); !strings.Contains(current, "Forest Clearing") {
		t.Errorf("venice miss should keep origin as current_location\n%s", current)
	}
	if !strings.Contains(dynamic, "<adjacent_previews>") {
		t.Error("venice miss should keep adjacent_previews")
	}

	unknown := new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeMovement,
		Object:  "The Moon",
	})
	messages, err = BuildNarratorMessages(gs, sc, "fly", 20, unknown)
	if err != nil {
		t.Fatalf("unknown dest: %v", err)
	}
	dynamic = messages[1].Content
	if strings.Contains(dynamic, "<destination>") {
		t.Error("unresolved location must not be injected")
	}
	if strings.Contains(dynamic, "The PC is arriving") {
		t.Error("unresolved destination should not invent an arrival line")
	}
	if current := currentLocationBlock(dynamic); !strings.Contains(current, "Forest Clearing") {
		t.Errorf("unresolved dest should keep origin as current_location\n%s", current)
	}
}
func TestBuildNarratorMessages_FollowingNPCOnMove(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	gs.WorldLocations = map[string]scenario.Location{
		"start": {Name: "Forest Clearing", Description: "A quiet glade.", Exits: map[string]string{"east": "cave"}},
		"cave":  {Name: "Dark Cave", Description: "A dripping limestone cave.", Exits: map[string]string{"west": "start"}},
	}
	gs.NPCs = map[string]*character.NPC{
		"pip": {Name: "Pip Upton", Location: "start", Following: "pc"},
	}
	sc := basicScenario()
	sc.Locations = gs.WorldLocations
	move := new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeMovement,
		Object:  "cave",
	})

	messages, err := BuildNarratorMessages(gs, sc, "go east", 20, move)
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	current := currentLocationBlock(messages[1].Content)
	if !strings.Contains(current, "Pip Upton") {
		t.Errorf("PC-following NPC should appear at dest\n%s", current)
	}
	if gs.NPCs["pip"].Location != "start" {
		t.Errorf("follower Location = %q, want start (Apply still owns the sync)", gs.NPCs["pip"].Location)
	}
}

func currentLocationBlock(s string) string {
	start := strings.Index(s, "<current_location>")
	end := strings.Index(s, "</current_location>")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return s[start:end]
}

func TestBuildNarratorMessages_FocusedNPCDescription(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "tavern"
	gs.NPCs = map[string]*character.NPC{
		"pip": {
			Name:        "Pip Upton",
			Description: "A cheerful deckhand.",
			Location:    "tavern",
		},
		"beatrice": {Name: "Sister Beatrice", Location: "tavern"},
	}
	gs.WorldLocations = map[string]scenario.Location{
		"tavern": {Name: "The Tavern", Description: "A smoky taproom."},
	}
	sc := basicScenario()
	sc.Locations = gs.WorldLocations
	ruling := new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeDialogue,
		Object:  "Pip Upton",
	})

	messages, err := BuildNarratorMessages(gs, sc, "talk to Pip", 20, ruling)
	if err != nil {
		t.Fatalf("BuildNarratorMessages: %v", err)
	}
	dynamic := messages[1].Content
	if !strings.Contains(dynamic, "- Pip Upton: A cheerful deckhand.") {
		t.Errorf("focused NPC should include description\n%s", dynamic)
	}
	if !strings.Contains(dynamic, "- Sister Beatrice\n") {
		t.Errorf("unfocused NPC should be a bare name\n%s", dynamic)
	}

	subject := new(chat.Ruling{
		Allowed: true,
		Scope:   chat.RulingScopeCombat,
		Subject: "Pip Upton",
		Object:  "Felix",
	})
	messages, err = BuildNarratorMessages(gs, sc, "Giant Rat strikes Felix.", 20, subject)
	if err != nil {
		t.Fatalf("subject focus: %v", err)
	}
	dynamic = messages[1].Content
	if !strings.Contains(dynamic, "- Pip Upton: A cheerful deckhand.") {
		t.Errorf("non-PC subject should expand NPC description\n%s", dynamic)
	}
}

func TestBuildNarratorMessages_WorldEventRuleConditional(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-provider", "test-model")
	gs.Location = "start"
	sc := basicScenario()

	messages, err := BuildNarratorMessages(gs, sc, "look", 20, nil)
	if err != nil {
		t.Fatalf("no story event: %v", err)
	}
	if strings.Contains(messages[0].Content, worldEventContinuationRule) {
		t.Error("world-event rule must not live in persistent")
	}
	if strings.Contains(messages[1].Content, worldEventContinuationRule) {
		t.Error("world-event rule should be omitted when history has no story event")
	}

	gs.ChatHistory = []chat.ChatMessage{
		{Role: chat.ChatRoleAgent, Content: "A horn sounds in the distance.", IsStoryEvent: true},
	}
	messages, err = BuildNarratorMessages(gs, sc, "look", 20, nil)
	if err != nil {
		t.Fatalf("with story event: %v", err)
	}
	if strings.Contains(messages[0].Content, worldEventContinuationRule) {
		t.Error("world-event rule must not live in persistent")
	}
	if !strings.Contains(messages[1].Content, worldEventContinuationRule) {
		t.Error("expected world-event rule when a story event is in the window")
	}

	// Aged out of the history window: rule should disappear.
	gs.ChatHistory = append(gs.ChatHistory,
		chat.ChatMessage{Role: chat.ChatRoleUser, Content: "I wait"},
		chat.ChatMessage{Role: chat.ChatRoleAgent, Content: "Time passes."},
	)
	messages, err = BuildNarratorMessages(gs, sc, "look", 2, nil)
	if err != nil {
		t.Fatalf("windowed out: %v", err)
	}
	if strings.Contains(messages[1].Content, worldEventContinuationRule) {
		t.Error("world-event rule should be omitted when the story event is outside the window")
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
