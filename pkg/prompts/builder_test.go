package prompts

import (
	"testing"

	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/conditionals"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

func TestNew(t *testing.T) {
	builder := New()
	if builder == nil {
		t.Fatal("Expected builder to be created, got nil")
	}
	if builder.historyLimit != 20 {
		t.Errorf("Expected default history limit of 20, got %d", builder.historyLimit)
	}
	if builder.messages == nil {
		t.Error("Expected messages slice to be initialized")
	}
}

func TestBuilder_FluentInterface(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")
	scenario := &scenario.Scenario{
		Name:   "Test",
		Story:  "A test story",
		Rating: scenario.RatingPG,
	}

	// Test that all methods return the builder for chaining
	builder := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Hello", chat.ChatRoleUser).
		WithHistoryLimit(10)

	if builder.gs != gs {
		t.Error("WithGameState did not set gamestate")
	}
	if builder.scenario != scenario {
		t.Error("WithScenario did not set scenario")
	}
	if builder.userMessage != "Hello" {
		t.Error("WithUserMessage did not set message")
	}
	if builder.userRole != chat.ChatRoleUser {
		t.Error("WithUserMessage did not set role")
	}
	if builder.historyLimit != 10 {
		t.Error("WithHistoryLimit did not set limit")
	}
}

func TestBuilder_Build_RequiresGameState(t *testing.T) {
	scenario := &scenario.Scenario{
		Name:   "Test",
		Story:  "A test story",
		Rating: scenario.RatingPG,
	}

	builder := New().WithScenario(scenario)
	_, err := builder.Build()

	if err == nil {
		t.Error("Expected error when gamestate is not set")
	}
	if err.Error() != "gamestate is required" {
		t.Errorf("Expected 'gamestate is required' error, got: %v", err)
	}
}

func TestBuilder_Build_RequiresScenario(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")

	builder := New().WithGameState(gs)
	_, err := builder.Build()

	if err == nil {
		t.Error("Expected error when scenario is not set")
	}
	if err.Error() != "scenario is required" {
		t.Errorf("Expected 'scenario is required' error, got: %v", err)
	}
}

func TestBuilder_Build_BasicMessages(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")
	gs.Location = "start"

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Hello world", chat.ChatRoleUser).
		Build()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have: system prompt, user message, final reminder
	if len(messages) < 3 {
		t.Errorf("Expected at least 3 messages, got %d", len(messages))
	}

	// First message should be system
	if messages[0].Role != chat.ChatRoleSystem {
		t.Errorf("Expected first message to be system, got %s", messages[0].Role)
	}

	// Last-1 message should be user
	if messages[len(messages)-2].Role != chat.ChatRoleUser {
		t.Errorf("Expected user message, got %s", messages[len(messages)-2].Role)
	}
	if messages[len(messages)-2].Content != "Hello world" {
		t.Errorf("Expected user message 'Hello world', got %s", messages[len(messages)-2].Content)
	}

	// Last message should be system (final prompt)
	if messages[len(messages)-1].Role != chat.ChatRoleSystem {
		t.Errorf("Expected final message to be system, got %s", messages[len(messages)-1].Role)
	}
}

func TestBuilder_Build_WithNarrator(t *testing.T) {
	narrator := &scenario.Narrator{
		ID:   "test_narrator",
		Name: "Test Narrator",
		Prompts: []string{
			"You are a test narrator.",
			"Speak in a test voice.",
		},
	}

	gs := state.NewGameState("test.json", narrator, "test-model")
	gs.Location = "start"

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Test", chat.ChatRoleUser).
		Build()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// First message should contain narrator prompts
	systemPrompt := messages[0].Content
	if !contains(systemPrompt, "You are a test narrator.") {
		t.Error("Expected system prompt to contain narrator prompts")
	}
}

func TestBuilder_Build_WithPC(t *testing.T) {
	pcSpec := &actor.PCSpec{
		ID:          "test_pc",
		Name:        "Test Character",
		Description: "A brave adventurer",
		HP:          10,
		MaxHP:       10,
		ContingencyPrompts: []conditionals.ContingencyPrompt{
			{
				Prompt: "You are playing as a test character.",
			},
		},
	}
	pc, err := actor.NewPCFromSpec(pcSpec)
	if err != nil {
		t.Fatalf("Failed to create PC: %v", err)
	}

	gs := state.NewGameState("test.json", nil, "test-model")
	gs.PC = pc
	gs.Location = "start"

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Test", chat.ChatRoleUser).
		Build()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// First message should contain PC information
	systemPrompt := messages[0].Content
	if !contains(systemPrompt, "Test Character") {
		t.Error("Expected system prompt to contain PC name")
	}
	// PC contingency prompts appear in the guidelines section
	if !contains(systemPrompt, "You are playing as a test character.") {
		t.Error("Expected system prompt to contain PC contingency prompts in guidelines section")
	}
}

func TestBuilder_Build_WithChatHistory(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")
	gs.Location = "start"
	gs.ChatHistory = []chat.ChatMessage{
		{Role: chat.ChatRoleUser, Content: "Message 1"},
		{Role: chat.ChatRoleAgent, Content: "Response 1"},
		{Role: chat.ChatRoleUser, Content: "Message 2"},
		{Role: chat.ChatRoleAgent, Content: "Response 2"},
	}

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Message 3", chat.ChatRoleUser).
		Build()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should include: system, 4 history messages, new user message, final prompt
	if len(messages) != 7 {
		t.Errorf("Expected 7 messages (1 system + 4 history + 1 user + 1 final), got %d", len(messages))
	}

	// Check history is included
	if messages[1].Content != "Message 1" {
		t.Error("Expected chat history to be included")
	}
}

func TestBuilder_Build_HistoryWindowing(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")
	gs.Location = "start"

	// Create more history than the limit
	for i := 0; i < 15; i++ {
		gs.ChatHistory = append(gs.ChatHistory, chat.ChatMessage{
			Role:    chat.ChatRoleUser,
			Content: "Message",
		})
	}

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Test", chat.ChatRoleUser).
		WithHistoryLimit(5). // Limit to 5 history messages
		Build()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have: system, 5 history (not 15), user message, final prompt
	if len(messages) != 8 {
		t.Errorf("Expected 8 messages (1 system + 5 history + 1 user + 1 final), got %d", len(messages))
	}
}

func TestBuilder_Build_WithStoryEvents(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")
	gs.Location = "start"
	gs.StoryEventQueue = []string{"Event 1", "Event 2"}

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Test", chat.ChatRoleUser).
		Build()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have story event message as a system message (not agent)
	found := false
	for _, msg := range messages {
		if msg.Role == chat.ChatRoleSystem && contains(msg.Content, "STORY EVENT") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected story events to be included in messages as system role")
	}
}

func TestBuilder_Build_GameEnded(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")
	gs.Location = "start"
	gs.IsEnded = true

	scenario := &scenario.Scenario{
		Name:          "Test Scenario",
		Story:         "A test adventure",
		Rating:        scenario.RatingPG,
		GameEndPrompt: "The adventure has ended!",
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Test", chat.ChatRoleUser).
		Build()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Last message should contain game end prompt
	lastMessage := messages[len(messages)-1]
	if !contains(lastMessage.Content, "The adventure has ended!") {
		t.Error("Expected final message to contain game end prompt")
	}
}

func TestBuilder_Build_WithContingencyPrompts(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")
	gs.Location = "start"
	gs.Vars = map[string]string{"test_flag": "true"}

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		ContingencyPrompts: []conditionals.ContingencyPrompt{
			{
				Prompt: "Always show this prompt",
			},
			{
				Prompt: "Show when flag is true",
				When: &conditionals.ConditionalWhen{
					Vars: map[string]string{"test_flag": "true"},
				},
			},
		},
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage("Test", chat.ChatRoleUser).
		Build()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// System prompt should contain contingency prompts
	systemPrompt := messages[0].Content
	if !contains(systemPrompt, "Always show this prompt") {
		t.Error("Expected system prompt to contain contingency prompts")
	}
	if !contains(systemPrompt, "Show when flag is true") {
		t.Error("Expected system prompt to contain conditional contingency prompts")
	}
}

func TestBuildMessages_ConvenienceFunction(t *testing.T) {
	gs := state.NewGameState("test.json", nil, "test-model")
	gs.Location = "start"

	scenario := &scenario.Scenario{
		Name:   "Test Scenario",
		Story:  "A test adventure",
		Rating: scenario.RatingPG,
		Locations: map[string]scenario.Location{
			"start": {
				Name:        "start",
				Description: "Starting location",
			},
		},
	}

	messages, err := BuildMessages(gs, scenario, "Test message", chat.ChatRoleUser, 10)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(messages) < 3 {
		t.Errorf("Expected at least 3 messages, got %d", len(messages))
	}

	// Should contain the user message
	found := false
	for _, msg := range messages {
		if msg.Role == chat.ChatRoleUser && msg.Content == "Test message" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find user message in built messages")
	}
}

func TestBuildMessages_ErrorHandling(t *testing.T) {
	// Test with nil gamestate
	_, err := BuildMessages(nil, &scenario.Scenario{}, "Test", chat.ChatRoleUser, 10)
	if err == nil {
		t.Error("Expected error with nil gamestate")
	}

	// Test with nil scenario
	gs := state.NewGameState("test.json", nil, "test-model")
	_, err = BuildMessages(gs, nil, "Test", chat.ChatRoleUser, 10)
	if err == nil {
		t.Error("Expected error with nil scenario")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
