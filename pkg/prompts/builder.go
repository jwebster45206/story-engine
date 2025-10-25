package prompts

import (
	"fmt"
	"strings"

	"github.com/jwebster45206/story-engine/pkg/chat"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// Builder constructs chat messages for LLM interaction using a fluent interface.
// It separates prompt building logic from game state management.
type Builder struct {
	gs           *state.GameState
	scenario     *scenario.Scenario
	userMessage  string
	userRole     string
	historyLimit int
	messages     []chat.ChatMessage
}

// New creates a new prompt builder with default settings.
func New() *Builder {
	return &Builder{
		historyLimit: 20, // default history limit
		messages:     make([]chat.ChatMessage, 0),
	}
}

// WithGameState sets the gamestate (contains embedded narrator and PC).
func (b *Builder) WithGameState(gs *state.GameState) *Builder {
	b.gs = gs
	return b
}

// WithScenario sets the scenario (loaded by handler on each request).
func (b *Builder) WithScenario(s *scenario.Scenario) *Builder {
	b.scenario = s
	return b
}

// WithUserMessage sets the user's message and role.
func (b *Builder) WithUserMessage(message string, role string) *Builder {
	b.userMessage = message
	b.userRole = role
	return b
}

// WithHistoryLimit sets the chat history window size.
func (b *Builder) WithHistoryLimit(limit int) *Builder {
	b.historyLimit = limit
	return b
}

// Build constructs and returns the final message array for LLM consumption.
func (b *Builder) Build() ([]chat.ChatMessage, error) {
	if b.gs == nil {
		return nil, fmt.Errorf("gamestate is required")
	}
	if b.scenario == nil {
		return nil, fmt.Errorf("scenario is required")
	}

	// Reset messages
	b.messages = make([]chat.ChatMessage, 0)

	// 1. System prompt
	if err := b.addSystemPrompt(); err != nil {
		return nil, fmt.Errorf("error building system prompt: %w", err)
	}

	// 2. Windowed chat history
	b.addHistory()

	// 3. User message
	b.addUserMessage()

	// 4. Story events (if any)
	b.addStoryEvents()

	// 5. Final reminders
	b.addFinalPrompt()

	return b.messages, nil
}

// addSystemPrompt builds the main system prompt from narrator, scenario, and state.
func (b *Builder) addSystemPrompt() error {
	var sb strings.Builder

	// Build system prompt with embedded narrator and PC
	systemPrompt := BuildSystemPrompt(b.gs.Narrator, b.gs.PC)
	sb.WriteString(systemPrompt)

	// Add rating prompt
	sb.WriteString("\n\nContent Rating: " + b.scenario.Rating)
	ratingPrompt := GetContentRatingPrompt(b.scenario.Rating)
	if ratingPrompt != "" {
		sb.WriteString(" (" + ratingPrompt + ")")
	}

	// Add state context
	statePrompt, err := GetStatePrompt(b.gs, b.scenario)
	if err != nil {
		return fmt.Errorf("error generating state prompt: %w", err)
	}
	sb.WriteString("\n\n" + statePrompt.Content)

	// Add contingency prompts
	contingencyPrompts := b.gs.GetContingencyPrompts(b.scenario)
	if len(contingencyPrompts) > 0 {
		sb.WriteString("\n\nSome important storytelling guidelines:\n\n")
		for i, prompt := range contingencyPrompts {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, prompt))
		}
	}

	b.messages = append(b.messages, chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: sb.String(),
	})

	return nil
}

// addHistory adds windowed chat history to the message array.
func (b *Builder) addHistory() {
	if len(b.gs.ChatHistory) == 0 {
		return
	}

	// Window the history to the specified limit
	if len(b.gs.ChatHistory) <= b.historyLimit {
		b.messages = append(b.messages, b.gs.ChatHistory...)
	} else {
		b.messages = append(b.messages, b.gs.ChatHistory[len(b.gs.ChatHistory)-b.historyLimit:]...)
	}
}

// addUserMessage adds the current user message to the message array.
func (b *Builder) addUserMessage() {
	if b.userMessage == "" {
		return
	}

	b.messages = append(b.messages, chat.ChatMessage{
		Role:    b.userRole,
		Content: b.userMessage,
	})
}

// addStoryEvents adds queued story events if present.
func (b *Builder) addStoryEvents() {
	storyEventPrompt := b.gs.GetStoryEvents()
	if storyEventPrompt == "" {
		return
	}

	b.messages = append(b.messages, chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: storyEventPrompt,
	})
}

// addFinalPrompt adds game-end or standard reminders.
func (b *Builder) addFinalPrompt() {
	var finalPrompt string

	if b.gs.IsEnded {
		// If the game is over, add the end prompt
		finalPrompt = GameEndSystemPrompt
		if b.scenario.GameEndPrompt != "" {
			finalPrompt += "\n\n" + b.scenario.GameEndPrompt
		}
	} else {
		finalPrompt = UserPostPrompt
	}

	b.messages = append(b.messages, chat.ChatMessage{
		Role:    chat.ChatRoleSystem,
		Content: finalPrompt,
	})
}

// BuildMessages is a convenience function for the common case.
// It creates a builder, sets all parameters, and builds the messages in one call.
func BuildMessages(
	gs *state.GameState,
	scenario *scenario.Scenario,
	message string,
	role string,
	historyLimit int,
) ([]chat.ChatMessage, error) {
	return New().
		WithGameState(gs).
		WithScenario(scenario).
		WithUserMessage(message, role).
		WithHistoryLimit(historyLimit).
		Build()
}
